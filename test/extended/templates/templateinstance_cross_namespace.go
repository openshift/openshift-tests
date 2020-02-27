package templates

import (
	"errors"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/test/e2e/framework"

	templatev1 "github.com/openshift/api/template/v1"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateinstance cross-namespace test", func() {
	defer g.GinkgoRecover()

	var (
		cli  = exutil.NewCLI("templates", exutil.KubeConfigPath())
		cli2 = exutil.NewCLI("templates2", exutil.KubeConfigPath())
	)

	g.It("should create and delete objects across namespaces", func() {
		g.Skip("Bug 1731222: skip template tests until we determine what is broken")
		err := cli2.AsAdmin().Run("adm").Args("policy", "add-role-to-user", "admin", cli.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// parameters for templateinstance
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Create(&kapiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "secret",
			},
			Data: map[string][]byte{
				"NAMESPACE": []byte(cli2.Namespace()),
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		templateinstance := &templatev1.TemplateInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name: "templateinstance",
			},
			Spec: templatev1.TemplateInstanceSpec{
				Template: templatev1.Template{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "template",
						Namespace: cli.Namespace(),
					},
					Parameters: []templatev1.Parameter{
						{
							Name: "NAMESPACE",
						},
					},
				},
				Secret: &corev1.LocalObjectReference{
					Name: "secret",
				},
			},
		}

		err = addObjectsToTemplate(&templateinstance.Spec.Template, []runtime.Object{
			// secret in the same namespace
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secret1",
				},
			},
			// secret in a different namespace
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret2",
					Namespace: "${NAMESPACE}",
				},
			},
		}, legacyscheme.Scheme.PrioritizedVersionsAllGroups()...)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating the templateinstance")
		_, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Create(templateinstance)
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for templateinstance controller to do its thing
		err = wait.Poll(time.Second, time.Minute, func() (bool, error) {
			templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			for _, c := range templateinstance.Status.Conditions {
				if c.Reason == "Failed" && c.Status == corev1.ConditionTrue {
					return false, fmt.Errorf("failed condition: %s", c.Message)
				}
				if c.Reason == "Created" && c.Status == corev1.ConditionTrue {
					return true, nil
				}
			}

			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.Logf("Template Instance object: %#v", templateinstance)
		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.KubeClient().CoreV1().Secrets(cli2.Namespace()).Get("secret2", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deleting the templateinstance")
		foreground := metav1.DeletePropagationForeground
		err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Delete(templateinstance.Name, &metav1.DeleteOptions{PropagationPolicy: &foreground})
		o.Expect(err).NotTo(o.HaveOccurred())

		// wait for garbage collector to do its thing
		err = wait.Poll(100*time.Millisecond, 30*time.Second, func() (bool, error) {
			_, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get(templateinstance.Name, metav1.GetOptions{})
			if kerrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = cli.KubeClient().CoreV1().Secrets(cli.Namespace()).Get("secret1", metav1.GetOptions{})
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())

		_, err = cli.KubeClient().CoreV1().Secrets(cli2.Namespace()).Get("secret2", metav1.GetOptions{})
		o.Expect(kerrors.IsNotFound(err)).To(o.BeTrue())
	})
})

// AddObjectsToTemplate adds the objects to the template using the target versions to choose the conversion destination
func addObjectsToTemplate(template *templatev1.Template, objects []runtime.Object, targetVersions ...schema.GroupVersion) error {
	for i := range objects {
		obj := objects[i]
		if obj == nil {
			return errors.New("cannot add a nil object to a template")
		}

		// We currently add legacy types first to the scheme, followed by the types in the new api
		// groups. We have to check all ObjectKinds and not just use the first one returned by
		// ObjectKind().
		gvks, _, err := legacyscheme.Scheme.ObjectKinds(obj)
		if err != nil {
			return err
		}

		var targetVersion *schema.GroupVersion
	outerLoop:
		for j := range targetVersions {
			possibleVersion := targetVersions[j]
			for _, kind := range gvks {
				if kind.Group == possibleVersion.Group {
					targetVersion = &possibleVersion
					break outerLoop
				}
			}
		}
		if targetVersion == nil {
			return fmt.Errorf("no target version found for object[%d], gvks %v in %v", i, gvks, targetVersions)
		}

		wrappedObject := runtime.NewEncodable(legacyscheme.Codecs.LegacyCodec(*targetVersion), obj)
		template.Objects = append(template.Objects, runtime.RawExtension{Object: wrappedObject})
	}

	return nil
}
