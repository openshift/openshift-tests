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
		currentPackage := CreateSubscriptionSpecificNamespace(kafkaPackageName, oc, true, true, namespace)

		CheckDeployment(currentPackage, oc)
		CreateCR(kafkaFile, oc)
		CheckCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		RemoveCR(currentPackage, kafkaCR, kafkaClusterName, oc)
		RemoveOperatorDependencies(currentPackage, oc, false)
		RemoveNamespace(currentPackage.Namespace, oc)
	})

})

func CreateCR(filename string, oc *exutil.CLI) {
	buildPruningBaseDir := exutil.FixturePath("testdata", "isv")
	cr := filepath.Join(buildPruningBaseDir, filename)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", cr).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}
func RemoveCR(p Packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	msg, err := oc.SetNamespace(p.Namespace).AsAdmin().Run("delete").Args(CRName, instanceName).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(msg).To(o.ContainSubstring("deleted"))
}

func CheckCR(p Packagemanifest, CRName string, instanceName string, oc *exutil.CLI) {
	poolErr := wait.Poll(10*time.Second, 300*time.Second, func() (bool, error) {
		msg, _ := oc.SetNamespace(p.Namespace).AsAdmin().Run("get").Args(CRName, instanceName, "-o=jsonpath={.status.conditions[0].type}").Output()
		if msg == "Ready" {
			return true, nil
		}
		return false, nil
	})
	if poolErr != nil {
		e2e.Logf("Could not get CR " + CRName + " for " + p.CsvVersion)
		RemoveCR(p, CRName, instanceName, oc)
		RemoveOperatorDependencies(p, oc, false)
		g.Fail("Could not get CR " + CRName + " for " + p.CsvVersion)
	}
}
