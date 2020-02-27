package templates

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	templatev1 "github.com/openshift/api/template/v1"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Conformance][templates] templateinstance creation with invalid object reports error", func() {
	defer g.GinkgoRecover()

	var (
		cli             = exutil.NewCLI("templates", exutil.KubeConfigPath())
		templatefixture = exutil.FixturePath("testdata", "templates", "templateinstance_badobject.yaml")
	)

	g.Context("", func() {
		g.It("should report a failure on creation", func() {
			g.Skip("Bug 1731222: skip template tests until we determine what is broken")
			err := cli.Run("create").Args("-f", templatefixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for error to appear")
			var templateinstance *templatev1.TemplateInstance
			err = wait.Poll(time.Second, 1*time.Minute, func() (bool, error) {
				templateinstance, err = cli.TemplateClient().TemplateV1().TemplateInstances(cli.Namespace()).Get("invalidtemplateinstance", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if TemplateInstanceHasCondition(templateinstance, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "error waiting for instantiate failure: %v\n%#v", err, templateinstance)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
