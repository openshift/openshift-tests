package isv

import (
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Suite:openshift/isv][Intermediate] Operator", func() {
	var (
		oc = exutil.NewCLI("isv", exutil.KubeConfigPath())
	)

	g.It("AMQ-Streams should work properly", func() {
		kafkaCR := "Kafka"
		kafkaClusterName := "my-cluster"
		kafkaPackageName := "amq-streams"
		kafkaFile := "kafka.yaml"
		namespace := "amq-streams"
		currentPackage := createSubscription(kafkaPackageName, oc, false, namespace)

		checkDeployment(currentPackage, oc)
		createCR(kafkaFile, oc)
		checkCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		removeCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		removeOperatorDependencies(currentPackage, oc, false)
		removeNamespace(currentPackage.namespace, oc)
	})

})

func createCR(filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "isv")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cr).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}
func removeCR(p packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	msg, err := oc.SetNamespace(p.namespace).AsAdmin().Run("delete").Args(CRName, instanceName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring("deleted"))
}

func checkCR(p packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	poolErr := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		msg, _ := oc.SetNamespace(p.namespace).AsAdmin().Run("get").Args(CRName, instanceName, "-o=jsonpath={.status.conditions[0].type}").Output()
		if msg == "Ready" {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		e2e.Logf("Could not get CR " + CRName + " for " + p.csvVersion)
		removeCR(p, CRName, instanceName, oc)
		removeOperatorDependencies(p, oc, false)
		g.Fail("Could not get CR " + CRName + " for " + p.csvVersion)
	}
}
