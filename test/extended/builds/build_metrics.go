package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-apps] Workloads", func() {
	defer g.GinkgoRecover()
	const (
		ns       = "openshift-controller-manager"
		hostport = "8443"
	)
	var (
		oc = exutil.NewCLIWithoutNamespace("default")
	)
	g.It("Medium-29780-Controller metrics reported from openshift-controller-manager[Serial]", func() {
		g.By("check controller metrics")
		token, err := oc.AsAdmin().WithoutNamespace().Run("sa").Args("-n", "openshift-monitoring", "get-token", "prometheus-k8s").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		foundMetrics := false
		podList, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Error listing pods: %v", err)
		}
		for _, p := range podList.Items {
			foundAvaliableDc := false
			foundFailDc := false
			foundCancelDc := false
			results, err := getBearerTokenURLViaPod(ns, p.Name, fmt.Sprintf("https://%s:%s/metrics", p.Status.PodIP, hostport), token)
			o.Expect(err).NotTo(o.HaveOccurred())
			foundAvaliableDc = strings.Contains(string(results), "openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"available\"}")
			foundFailDc = strings.Contains(string(results), "openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"cancelled\"}")
			foundCancelDc = strings.Contains(string(results), "openshift_apps_deploymentconfigs_complete_rollouts_total{phase=\"failed\"}")
			if foundAvaliableDc && foundFailDc && foundCancelDc {
				foundMetrics = true
				break
			}
		}
		o.Expect(foundMetrics).To(o.BeTrue())
	})
})

func getBearerTokenURLViaPod(ns string, execPodName string, url string, bearer string) (string, error) {
	cmd := fmt.Sprintf("curl -s -k -H 'Authorization: Bearer %s' %q", bearer, url)
	output, err := e2e.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	return output, nil
}
