package router

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		configPath = exutil.FixturePath("testdata", "router", "reencrypt-serving-cert.yaml")
		oc         *exutil.CLI

		ip, ns string
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	oc = exutil.NewCLI("router-reencrypt", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		var err error
		ip, err = exutil.WaitForRouterServiceIP(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		ns = oc.KubeFramework().Namespace.Name
	})

	g.Describe("The HAProxy router", func() {
		g.It("should support reencrypt to services backed by a serving certificate automatically", func() {
			routerURL := fmt.Sprintf("https://%s", ip)

			execPodName := exutil.CreateExecPodOrFail(oc.AdminKubeClient().CoreV1(), ns, "execpod")
			defer func() { oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()
			g.By(fmt.Sprintf("deploying a service using a reencrypt route without a destinationCACertificate"))
			err := oc.Run("create").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var hostname string
			err = wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				route, err := oc.RouteClient().RouteV1().Routes(ns).Get("serving-cert", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(route.Status.Ingress) == 0 || len(route.Status.Ingress[0].Host) == 0 {
					return false, nil
				}
				hostname = route.Status.Ingress[0].Host
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// don't assume the router is available via external DNS, because of complexity
			err = waitForRouterOKResponseExec(ns, execPodName, routerURL, hostname, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
