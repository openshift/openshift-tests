package project

import (
	"time"

	"k8s.io/client-go/util/retry"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	projectv1 "github.com/openshift/api/project/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[Feature:ProjectAPI] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api", exutil.KubeConfigPath())

	g.It("TestUnprivilegedNewProject", func() {
		t := g.GinkgoT()

		valerieProjectClient := oc.ProjectClient()

		// confirm that we have access to request the project
		allowed := &metav1.Status{}
		if err := valerieProjectClient.ProjectV1().RESTClient().Get().Resource("projectrequests").Do().Into(allowed); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed.Status != metav1.StatusSuccess {
			t.Fatalf("expected %v, got %v", metav1.StatusSuccess, allowed.Status)
		}

		projectRequest := &projectv1.ProjectRequest{}
		projectRequest.Name = "new-project-" + oc.Namespace()
		projectRequest.DisplayName = "display name here"
		projectRequest.Description = "the special description"
		projectRequest.Annotations = make(map[string]string)

		project, err := valerieProjectClient.ProjectV1().ProjectRequests().Create(projectRequest)
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.AddResourceToDelete(projectv1.GroupVersion.WithResource("projects"), project)

		waitForProject(t, valerieProjectClient.ProjectV1(), projectRequest.Name, 5*time.Second, 10)

		actualProject, err := valerieProjectClient.ProjectV1().Projects().Get(projectRequest.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if e, a := oc.Username(), actualProject.Annotations["openshift.io/requester"]; e != a {
			t.Errorf("incorrect project requester: expected %v, got %v", e, a)
		}

		if _, err := valerieProjectClient.ProjectV1().ProjectRequests().Create(projectRequest); !kapierrors.IsAlreadyExists(err) {
			t.Fatalf("expected an already exists error, but got %v", err)
		}

	})
})

var _ = g.Describe("[Feature:ProjectAPI][Serial] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("project-api", exutil.KubeConfigPath())

	g.It("TestUnprivilegedNewProjectDenied", func() {
		t := g.GinkgoT()

		clusterAdminAuthorizationConfig := oc.AdminAuthorizationClient().AuthorizationV1()
		role, err := clusterAdminAuthorizationConfig.ClusterRoles().Get("self-provisioner", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		existingRole := role.DeepCopy()
		defer func() {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				currentRole, err := clusterAdminAuthorizationConfig.ClusterRoles().Get("self-provisioner", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				currentRole.Rules = existingRole.Rules
				_, err = clusterAdminAuthorizationConfig.ClusterRoles().Update(currentRole)
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		role.Rules = []authorizationv1.PolicyRule{}
		_, err = clusterAdminAuthorizationConfig.ClusterRoles().Update(role)
		o.Expect(err).NotTo(o.HaveOccurred())

		valerieProjectClient := oc.ProjectClient()
		err = oc.WaitForAccessDenied(&kubeauthorizationv1.SelfSubjectAccessReview{
			Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
					Verb:     "create",
					Group:    "project.openshift.io",
					Resource: "projectrequests",
				},
			},
		}, oc.Username())
		o.Expect(err).NotTo(o.HaveOccurred())

		// confirm that we have access to request the project
		err = valerieProjectClient.ProjectV1().RESTClient().Get().Resource("projectrequests").Do().Into(&metav1.Status{})
		o.Expect(err).To(o.HaveOccurred())
		expectedError := `You may not request a new project via this API.`
		if (err != nil) && (err.Error() != expectedError) {
			t.Fatalf("expected\n\t%v\ngot\n\t%v", expectedError, err.Error())
		}
	})
})

// waitForProject will execute a client list of projects looking for the project with specified name
// if not found, it will retry up to numRetries at the specified delayInterval
func waitForProject(t g.GinkgoTInterface, client projectv1client.ProjectV1Interface, projectName string, delayInterval time.Duration, numRetries int) {
	for i := 0; i <= numRetries; i++ {
		_, err := client.Projects().Get(projectName, metav1.GetOptions{})
		if err == nil {
			return
		}
		time.Sleep(delayInterval)
	}
	t.Errorf("expected project %v not found", projectName)
}
