package operators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/github"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"path/filepath"
	"strings"
	"time"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:Platform] OLM should", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("default")

	operators := "operators.coreos.com"
	providedAPIs := []struct {
		fromAPIService bool
		group          string
		version        string
		plural         string
	}{
		{
			fromAPIService: true,
			group:          "packages." + operators,
			version:        "v1",
			plural:         "packagemanifests",
		},
		{
			group:   operators,
			version: "v1",
			plural:  "operatorgroups",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "clusterserviceversions",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "catalogsources",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "installplans",
		},
		{
			group:   operators,
			version: "v1alpha1",
			plural:  "subscriptions",
		},
	}

	for i := range providedAPIs {
		api := providedAPIs[i]
		g.It(fmt.Sprintf("be installed with %s at version %s", api.plural, api.version), func() {
			if api.fromAPIService {
				// Ensure spec.version matches expected
				raw, err := oc.AsAdmin().Run("get").Args("apiservices", fmt.Sprintf("%s.%s", api.version, api.group), "-o=jsonpath={.spec.version}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.Equal(api.version))
			} else {
				// Ensure expected version exists in spec.versions and is both served and stored
				raw, err := oc.AsAdmin().Run("get").Args("crds", fmt.Sprintf("%s.%s", api.plural, api.group), fmt.Sprintf("-o=jsonpath={.spec.versions[?(@.name==\"%s\")]}", api.version)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(raw).To(o.ContainSubstring("served:true"))
				o.Expect(raw).To(o.ContainSubstring("storage:true"))
			}
		})
	}

	// OCP-24061 - [bz 1685230] OLM operator should use imagePullPolicy: IfNotPresent
	// author: bandrade@redhat.com
	g.It("have imagePullPolicy:IfNotPresent on thier deployments", func() {
		deploymentResource := []string{"catalog-operator", "olm-operator", "packageserver"}
		for _, v := range deploymentResource {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "deployment", v, "-o=jsonpath={.spec.template.spec.containers[*].imagePullPolicy}").Output()
			e2e.Logf("%s.imagePullPolicy:%s", v, msg)
			if err != nil {
				e2e.Failf("Unable to get %s, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.Equal("IfNotPresent"))
		}
	})

	// OCP-21082 - Implement packages API server and list packagemanifest info with namespace not NULL
	// author: bandrade@redhat.com
	g.It("Implement packages API server and list packagemanifest info with namespace not NULL", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		packageserverLines := strings.Split(msg, "\n")
		if len(packageserverLines) > 0 {
			packageserverLine := strings.Fields(packageserverLines[0])
			if strings.Index(packageserverLines[0], packageserverLine[0]) != 0 {
				e2e.Failf("It should display a namespace for CSV: %s [ref:bz1670311]", packageserverLines[0])
			}
		} else {
			e2e.Failf("No packages for evaluating if package namespace is not NULL")
		}
	})

	// OCP-20981, [BZ 1626434]The olm/catalog binary should output the exact version info
	// author: jiazha@redhat.com
	g.It("[Serial] olm version should contain the source commit id", func() {
		sameCommit := ""
		subPods := []string{"catalog-operator", "olm-operator", "packageserver"}

		for _, v := range subPods {
			podName, err := oc.AsAdmin().Run("get").Args("-n", "openshift-operator-lifecycle-manager", "pods", "-l", fmt.Sprintf("app=%s", v), "-o=jsonpath={.items[0].metadata.name}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("get pod name:%s", podName)

			g.By(fmt.Sprintf("get olm version from the %s pod", v))
			oc.SetNamespace("openshift-operator-lifecycle-manager")
			commands := []string{"exec", podName, "--", "olm", "--version"}
			olmVersion, err := oc.AsAdmin().Run(commands...).Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			idSlice := strings.Split(olmVersion, ":")
			gitCommitID := strings.TrimSpace(idSlice[len(idSlice)-1])
			e2e.Logf("olm source git commit ID:%s", gitCommitID)
			if len(gitCommitID) != 40 {
				e2e.Failf(fmt.Sprintf("the length of the git commit id is %d, != 40", len(gitCommitID)))
			}

			if sameCommit == "" {
				sameCommit = gitCommitID
				g.By("checking this commitID in the operator-lifecycle-manager repo")
				client := github.NewClient(nil)
				_, _, err := client.Git.GetCommit(context.Background(), "operator-framework", "operator-lifecycle-manager", gitCommitID)
				if err != nil {
					e2e.Failf("Git.GetCommit returned error: %v", err)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

			} else if gitCommitID != sameCommit {
				e2e.Failf("These commitIDs inconformity!!!")
			}
		}
	})
})

// This context will cover test case: OCP-23440, author: jiazha@redhat.com
var _ = g.Describe("[Feature:Platform] an end user use OLM", func() {
	defer g.GinkgoRecover()

	var (
		oc           = exutil.NewCLI("olm-23440", exutil.KubeConfigPath())
		operatorWait = 120 * time.Second

		buildPruningBaseDir = exutil.FixturePath("testdata", "olm")
		operatorGroup       = filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
		etcdSub             = filepath.Join(buildPruningBaseDir, "etcd-subscription.yaml")
		etcdCluster         = filepath.Join(buildPruningBaseDir, "etcd-cluster.yaml")
	)

	files := []string{operatorGroup, etcdSub}
	g.It("can subscribe to the etcd operator", func() {
		g.By("Cluster-admin user subscribe the operator resource")
		for _, v := range files {
			configFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", v, "-p", "NAME=test-operator", fmt.Sprintf("NAMESPACE=%s", oc.Namespace()), "SOURCENAME=community-operators", "SOURCENAMESPACE=openshift-marketplace").OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

		}
		err := wait.Poll(10*time.Second, operatorWait, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("-n", oc.Namespace(), "csv", "etcdoperator.v0.9.4", "-o=jsonpath={.status.phase}").Output()
			if err != nil {
				e2e.Failf("Failed to deploy etcdoperator.v0.9.4, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "Succeeded") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Switch to common user to create the resources provided by the operator")
		etcdClusterName := "example-etcd-cluster"
		configFile, err := oc.Run("process").Args("-f", etcdCluster, "-p", fmt.Sprintf("NAME=%s", etcdClusterName), fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).OutputToFile("config.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", configFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(10*time.Second, operatorWait, func() (bool, error) {
			output, err := oc.Run("get").Args("-n", oc.Namespace(), "etcdCluster", etcdClusterName, "-o=jsonpath={.status}").Output()
			if err != nil {
				e2e.Failf("Failed to get etcdCluster, error:%v", err)
				return false, err
			}
			if strings.Contains(output, "phase:Running") && strings.Contains(output, "currentVersion:3.2.13") && strings.Contains(output, "size:3") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.Run("get").Args("pods", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring(etcdClusterName))
	})

	// OCP-24829 - Report `Upgradeable` in OLM ClusterOperators status
	// author: bandrade@redhat.com
	g.It("Report Upgradeable in OLM ClusterOperators status", func() {
		olmCOs := []string{"operator-lifecycle-manager", "operator-lifecycle-manager-catalog", "operator-lifecycle-manager-packageserver"}
		for _, co := range olmCOs {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", co, "-o=jsonpath={range .status.conditions[*]}{.type}{' '}{.status}").Output()
			if err != nil {
				e2e.Failf("Unable to get co %s status, error:%v", msg, err)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(msg).To(o.ContainSubstring("Upgradeable True"))
		}
	})

	// OCP-24818 - Checking OLM descriptors
	// author: tbuskey@redhat.com
	g.It("Checking OLM descriptors", func() {
		olmErr := 0
		olmErrDescriptor := []string{""}
		olmExplains := []string{"InstallPlan", "ClusterServiceVersion", "Subscription", "CatalogSource", "OperatorSource", "OperatorGroup", "PackageManifest"}
		for _, olmExplain := range olmExplains {
			msg, err := oc.AsAdmin().WithoutNamespace().Run("explain").Args(olmExplain).Output()
			if err != nil {
				olmErr++
				olmErrDescriptor = append(olmErrDescriptor, olmExplain)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(msg, "<empty>") {
				olmErr++
				olmErrDescriptor = append(olmErrDescriptor, olmExplain)
			}
		}
		if olmErr != 0 {
			// fmt.Printf("explain errors: %d\n", olmErr)
			e2e.Failf("%v errors in explaining the following OLM descriptors: %v", olmErr, olmErrDescriptor)
		}
	})

	// OCP-21953 Ensure that operator deployment is in the master node
	// author: tbuskey@redhat.com
	g.It("Ensure that operator deployment is in the master node", func() {
		olmErrs := true
		olmPodName := "marketplace-operator"
		olmNamespace := "openshift-marketplace"
		olmJpath := "-o=jsonpath={@.spec.template.spec.nodeSelector}"
		nodeRole := "node-role.kubernetes.io/master"
		olmMasterNode := false
		olmPodFullName := ""
		olmNodeName := ""
		pods := ""
		nodes := ""
		nodeStatus := false

		// Look at deployment for the marketplace-operator
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "-n", olmNamespace, olmPodName, olmJpath).Output()
		if err != nil {
			e2e.Failf("Unable to get deployment -n %v %v %v.", olmNamespace, olmPodName, olmJpath)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		// o.Expect(msg).NotTo(o.BeEmpty)
		if strings.Contains(msg, nodeRole) {
			olmMasterNode = true
		}
		if len(msg) < 1 || !olmMasterNode {
			e2e.Failf("Could not find %v variable %v for %v: %v", olmJpath, nodeRole, olmPodName, msg)
		}
		// look for the marketplace-operator pod's full name
		pods, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", olmNamespace, "-o", "wide").Output()
		if err != nil {
			e2e.Failf("Unable to query pods -n %v %v %v.", olmNamespace, err, pods)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods).NotTo(o.ContainSubstring("No resources found"))
		for _, pod := range strings.Split(pods, "\n") {
			if len(pod) <= 0 {
				continue
			}
			// Find the node in the pod
			if strings.Contains(pod, olmPodName) {
				x := strings.Fields(pod)
				olmPodFullName = x[0]
				olmNodeName = x[6]
				olmErrs = false
			}
		}
		if olmErrs {
			e2e.Failf("Unable to find the full pod name for %v in %v: %v.", olmPodName, olmNamespace, pods)
		}
		// Look at the setting for the node to be on the master
		olmErrs = true
		olmJpath = fmt.Sprintf("-o=go-template=$'{{index .metadata.labels \"%v\"}}'", nodeRole)
		nodes, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-n", olmNamespace, olmNodeName, olmJpath).Output()
		if err != nil {
			e2e.Failf("Unable to query nodes -n %v %v %v.", olmNamespace, err, nodes)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes).NotTo(o.ContainSubstring("No resources found"))
		// if nodes has no value, the variable was not set, so fail
		if nodes == "$'<no value>'" {
			e2e.Failf("The %v node of pod %v does not have %v set", olmNodeName, olmPodFullName, nodeRole)
		}
		// Found the setting, verify that it's really on the master node
		msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-n", olmNamespace, olmNodeName, "--show-labels", "--no-headers").Output()
		if err != nil {
			e2e.Failf("Unable to query the %v node of pod %v for %v's status", olmNodeName, olmPodFullName, err, msg)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.ContainSubstring("No resources found"))
		statuses := strings.Fields(msg)
		for _, status := range statuses {
			if status == "master" {
				olmErrs = false
				nodeStatus = true
			}
		}
		if olmErrs || !nodeStatus {
			e2e.Failf("The node %v of %v pod is not on master:%v", olmNodeName, olmPodFullName, msg)
		}
		e2e.Logf("The node %v of %v pod is on the master node", olmNodeName, olmPodFullName)
	})
  
	// OCP-27589 do not use ipv4 addresses in CatalogSources generated by marketplace
	// author: tbuskey@redhat.com
	g.It("do not use ipv4 addresses in CatalogSources generated by marketplace", func() {
		re := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
		olmErrs := 0
		olmNames := []string{""}
		olmNamespace := "openshift-marketplace"
		olmJpath := "-o=jsonpath={range .items[*]}{@.metadata.name}{','}{@.spec.address}{'\\n'}"
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-n", olmNamespace, olmJpath).Output()
		if err != nil {
			e2e.Failf("Unable to get pod -n %v %v.", olmNamespace, olmJpath)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(msg).NotTo(o.ContainSubstring("No resources found"))
		// msg = fmt.Sprintf("%v\ntest,1.1.1.1\n", msg)
		lines := strings.Split(msg, "\n")
		for _, line := range lines {
			if len(line) <= 0 {
				continue
			}
			name := strings.Split(line, ",")
			cscAddr := strings.Split(name[1], ":")[0]
			if re.MatchString(cscAddr) {
				olmErrs++
				olmNames = append(olmNames, name[0])
			}
		}
		if olmErrs > 0 {
			e2e.Failf("%v ipv4 addresses found in these OLM components: %v", olmErrs, olmNames)
		}
	})

	// OCP-21130 - [bug ALM-736] Fetching non-existent `PackageManifest` should return 404
	// author: bandrade@redhat.com
	g.It("Fetching non-existent `PackageManifest` should return 404", func() {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "--all-namespaces", "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		packageserverLines := strings.Split(msg, "\n")
		if len(packageserverLines) > 0 {
			raw, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "a_package_that_not_exists", "-o yaml", "--loglevel=8").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(raw).To(o.ContainSubstring("\"code\": 404"))
		} else {
			e2e.Failf("No packages to evaluate if 404 works when a PackageManifest does not exists")
		}
	})
})
