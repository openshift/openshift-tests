package builds

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildv1 "github.com/openshift/api/build/v1"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

// e2e tests of the build controller configuration.
// These are tagged [Serial] because each test modifies the cluster-wide build controller config.
var _ = g.Describe("[Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture              = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		buildFixture2             = exutil.FixturePath("testdata", "builds", "test-build-cluster-config.yaml")
		defaultConfigFixture      = exutil.FixturePath("testdata", "builds", "cluster-config.yaml")
		blacklistConfigFixture    = exutil.FixturePath("testdata", "builds", "cluster-config", "registry-blacklist.yaml")
		whitelistConfigFixture    = exutil.FixturePath("testdata", "builds", "cluster-config", "registry-whitelist.yaml")
		invalidproxyConfigFixture = exutil.FixturePath("testdata", "builds", "cluster-config", "invalid-build-cluster-config.yaml")
		oc                        = exutil.NewCLI("build-cluster-config", exutil.KubeConfigPath())
		checkPodProxyEnvs         = func(containers []v1.Container, proxySpec *configv1.ProxySpec) {
			o.Expect(containers).NotTo(o.BeNil())
			foundHTTP := false
			foundHTTPS := false
			foundNoProxy := false
			for _, container := range containers {
				o.Expect(container.Env).NotTo(o.BeNil())
				for _, env := range container.Env {
					switch {
					case env.Name == "HTTP_PROXY" && env.Value == proxySpec.HTTPProxy:
						foundHTTP = true
					case env.Name == "HTTPS_PROXY" && env.Value == proxySpec.HTTPSProxy:
						foundHTTPS = true
					case env.Name == "NO_PROXY" && env.Value == proxySpec.NoProxy:
						foundNoProxy = true
					}
				}
			}
			o.Expect(foundHTTP).To(o.BeTrue())
			o.Expect(foundHTTPS).To(o.BeTrue())
			o.Expect(foundNoProxy).To(o.BeTrue())
		}
		checkOCMProgressing = func(progressing operatorv1.ConditionStatus) {
			g.By("check that the OCM enters Progressing==" + string(progressing))
			var err error
			err = wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
				ocm, err := oc.AdminOperatorClient().OperatorV1().OpenShiftControllerManagers().Get("cluster", metav1.GetOptions{})
				if err != nil {
					g.By("intermediate error accessing ocm: " + err.Error())
					return false, nil
				}
				for _, c := range ocm.Status.Conditions {
					if c.Type == operatorv1.OperatorStatusTypeProgressing && c.Status == progressing {
						return true, nil
					}
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		checkDSRolloutState = func(inProgress bool) {
			g.By(fmt.Sprintf("check that a OCM DS rollout being in progress is %v", inProgress))
			var err error
			err = wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
				ds, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-controller-manager").Get("controller-manager", metav1.GetOptions{})
				if err != nil {
					g.By("intermediate error access ds: " + err.Error())
					return false, nil
				}
				if inProgress {
					if ds.Status.CurrentNumberScheduled < ds.Status.DesiredNumberScheduled ||
						ds.Status.NumberReady < ds.Status.DesiredNumberScheduled ||
						ds.Status.NumberAvailable < ds.Status.DesiredNumberScheduled ||
						ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled {
						return true, nil
					}
					return false, nil
				}
				if ds.Status.CurrentNumberScheduled == ds.Status.DesiredNumberScheduled &&
					ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
					ds.Status.NumberAvailable == ds.Status.DesiredNumberScheduled &&
					ds.Status.UpdatedNumberScheduled == ds.Status.DesiredNumberScheduled {
					return true, nil
				}
				e2e.Logf("ocm Desired: %d, Current: %d, Ready: %d, Available: %d, Updated: %d",
					ds.Status.DesiredNumberScheduled,
					ds.Status.CurrentNumberScheduled,
					ds.Status.NumberReady,
					ds.Status.NumberAvailable,
					ds.Status.UpdatedNumberScheduled)
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		checkBuildPodUnschedulable = func(name string) {
			g.By(fmt.Sprintf("check the build pod %s is unschedulable", name))
			var err error
			err = wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(name, metav1.GetOptions{})
				if err != nil {
					g.By("intermediate error access pod: " + err.Error())
					return false, nil
				}
				for _, c := range pod.Status.Conditions {
					if c.Type == v1.PodScheduled && c.Reason == v1.PodReasonUnschedulable {
						return true, nil
					}
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		getBuildFromPod = func(pod *v1.Pod) *buildv1.Build {
			o.Expect(len(pod.Spec.Containers)).NotTo(o.Equal(0))
			// borrowed from github.com/openshift/openshift-controller-manager/blob/master/pkg/build/controller/common/buildpodutil.go
			// since openshift-controller-manager is no longer vendored into openshift/origin
			annotationDecodingScheme := runtime.NewScheme()
			buildEnvVar := ""
			for _, envVar := range pod.Spec.Containers[0].Env {
				if envVar.Name == "BUILD" {
					buildEnvVar = envVar.Value
					break
				}
			}
			o.Expect(len(buildEnvVar)).NotTo(o.Equal(0))
			err := buildv1.Install(annotationDecodingScheme)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = buildv1.DeprecatedInstallWithoutGroup(annotationDecodingScheme)
			o.Expect(err).NotTo(o.HaveOccurred())
			annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
			decoder := annotationDecoderCodecFactory.UniversalDecoder(buildv1.GroupVersion)
			obj, err := runtime.Decode(decoder, []byte(buildEnvVar))
			o.Expect(err).NotTo(o.HaveOccurred())
			build, ok := obj.(*buildv1.Build)
			o.Expect(ok).To(o.BeTrue())
			return build
		}
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()
			oc.Run("create").Args("-f", buildFixture2).Execute()
		})

		g.JustAfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpConfigMapStates(oc)
			}
		})

		g.Context("registries config context", func() {

			// Altering registries config does not force an OCM rollout
			g.AfterEach(func() {
				oc.AsAdmin().Run("apply").Args("-f", defaultConfigFixture).Execute()
			})

			g.It("should default registry search to docker.io for image pulls", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply default cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", defaultConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build and waiting for success")
				// Image used by sample-build (centos/ruby-25-centos7) is only available on docker.io
				br, err := exutil.StartBuildAndWait(oc, "sample-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("expecting the build logs to indicate docker.io was the default registry")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("defaulting registry to docker.io"))
			})

			g.It("should allow registries to be blacklisted", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply blacklist cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", blacklistConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build-docker-args-preset and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate the image was rejected")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Source image rejected"))
			})

			g.It("should allow registries to be whitelisted", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply whitelist cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", whitelistConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build-docker-args-preset and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate the image was rejected")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Source image rejected"))
			})

		})

		g.Context("build config no ocm rollout", func() {
			g.AfterEach(func() {
				g.By("reset build cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildDefaults = configv1.BuildDefaults{}
				buildConfig.Spec.BuildOverrides = configv1.BuildOverrides{}
				_, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("Apply default proxy configuration to source build pod through env vars", func() {
				g.By("apply proxy cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", invalidproxyConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 10s for build controller to detect proxy cfg chg")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildDefaults.DefaultProxy).NotTo(o.BeNil())
				g.By("starting build verbose-s2i-build and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "verbose-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate invalid proxy")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("proxyconnect tcp: dial tcp: lookup invalid.proxy.redhat.com"))
				g.By("checking pod as well")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPodProxyEnvs(pod.Spec.Containers, buildConfig.Spec.BuildDefaults.DefaultProxy)
				checkPodProxyEnvs(pod.Spec.InitContainers, buildConfig.Spec.BuildDefaults.DefaultProxy)
			})

			g.It("Apply default proxy configuration to docker build pod through env vars", func() {
				g.By("apply proxy cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", invalidproxyConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 10s for build controller to detect proxy cfg chg")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildDefaults.DefaultProxy).NotTo(o.BeNil())
				g.By("starting build simple-s2i-build and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate invalid proxy")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Could not resolve proxy: invalid.proxy.redhat.com; Unknown error"))
				g.By("checking pod as well")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPodProxyEnvs(pod.Spec.Containers, buildConfig.Spec.BuildDefaults.DefaultProxy)
				checkPodProxyEnvs(pod.Spec.InitContainers, buildConfig.Spec.BuildDefaults.DefaultProxy)
			})

			// this replaces coverage from the TestBuildDefaultGitHTTPProxy and TestBuildDefaultGitHTTPSProxy integration test
			g.It("Apply git proxy configuration to build pod", func() {
				g.By("apply proxy cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildDefaults.GitProxy = &configv1.ProxySpec{
					HTTPProxy:  "http://invalid.proxy.redhat.com:3288",
					HTTPSProxy: "https://invalid.proxy.redhat.com:3288",
					NoProxy:    "image-registry.openshift-image-registry.svc:5000",
				}
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 10s for build controller to detect proxy cfg chg")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildDefaults.GitProxy).NotTo(o.BeNil())

				g.By("starting build simple-s2i-build and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate invalid proxy")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Could not resolve proxy: invalid.proxy.redhat.com; Unknown error"))
				g.By("checking build stored in pod as well")
				// note, only the build stored in the Pod's "BUILD" env var has the updated proxy settings; they do not
				// get propagated to the associated build stored in etcd
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				build := getBuildFromPod(pod)
				o.Expect(build.Spec.Source.Git).NotTo(o.BeNil())
				o.Expect(*build.Spec.Source.Git.HTTPProxy).To(o.Equal(buildConfig.Spec.BuildDefaults.GitProxy.HTTPProxy))
				o.Expect(*build.Spec.Source.Git.HTTPSProxy).To(o.Equal(buildConfig.Spec.BuildDefaults.GitProxy.HTTPSProxy))
				o.Expect(*build.Spec.Source.Git.NoProxy).To(o.Equal(buildConfig.Spec.BuildDefaults.GitProxy.NoProxy))
			})
		})

		g.Context("build config with ocm rollout", func() {

			g.AfterEach(func() {
				g.By("reset build cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildDefaults = configv1.BuildDefaults{}
				buildConfig.Spec.BuildOverrides = configv1.BuildOverrides{}
				_, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
			})

			// this replaces coverage from the TestBuildDefaultEnvironment integration test
			g.It("Apply env configuration to build pod", func() {
				g.By("apply env cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildDefaults.Env = []v1.EnvVar{
					{
						Name:  "VAR1",
						Value: "VALUE1",
					},
					{
						Name:  "VAR2",
						Value: "VALUE2",
					},
				}
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
				// there is no way programmatically to check leader election outcome
				g.By("waiting 10s for controller-manager leader election to complete")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildDefaults.Env).NotTo(o.BeNil())
				o.Expect(len(buildConfig.Spec.BuildDefaults.Env)).To(o.Equal(2))
				name1 := buildConfig.Spec.BuildDefaults.Env[0].Name
				value1 := buildConfig.Spec.BuildDefaults.Env[0].Value
				name2 := buildConfig.Spec.BuildDefaults.Env[1].Name
				value2 := buildConfig.Spec.BuildDefaults.Env[1].Value

				g.By("starting build simple-s2i-build and waiting for completion")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("checking build obj env field")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				build := getBuildFromPod(pod)
				o.Expect(build.Spec.Strategy.SourceStrategy).NotTo(o.BeNil())
				o.Expect(build.Spec.Strategy.SourceStrategy.Env).NotTo(o.BeNil())
				foundOne := false
				foundTwo := false
				for _, env := range build.Spec.Strategy.SourceStrategy.Env {
					switch {
					case env.Name == name1 && env.Value == value1:
						foundOne = true
					case env.Name == name2 && env.Value == value2:
						foundTwo = true
					}
				}
				o.Expect(foundOne).To(o.BeTrue())
				o.Expect(foundTwo).To(o.BeTrue())
			})

			// this replaces coverage from the TestBuildDefaultLabels integration test
			g.It("Apply default image label configuration to build pod", func() {
				g.By("apply label cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildDefaults.ImageLabels = []configv1.ImageLabel{
					{
						Name:  "VAR1",
						Value: "VALUE1",
					},
					{
						Name:  "VAR2",
						Value: "VALUE2",
					},
				}
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
				// there is no way programmatically to check leader election outcome
				g.By("waiting 10s for controller-manager leader election to complete")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildDefaults.ImageLabels).NotTo(o.BeNil())
				o.Expect(len(buildConfig.Spec.BuildDefaults.ImageLabels)).To(o.Equal(2))
				name1 := buildConfig.Spec.BuildDefaults.ImageLabels[0].Name
				value1 := buildConfig.Spec.BuildDefaults.ImageLabels[0].Value
				name2 := buildConfig.Spec.BuildDefaults.ImageLabels[1].Name
				value2 := buildConfig.Spec.BuildDefaults.ImageLabels[1].Value

				g.By("starting build simple-s2i-build and waiting for completion")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("checking build stored in pod as well")
				// note, only the build stored in the Pod's "BUILD" env var has the updated proxy settings; they do not
				// get propagated to the associated build stored in etcd
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				build := getBuildFromPod(pod)
				o.Expect(build.Spec.Output.ImageLabels).NotTo(o.BeNil())
				foundOne := false
				foundTwo := false
				for _, imglbl := range build.Spec.Output.ImageLabels {
					switch {
					case imglbl.Name == name1 && imglbl.Value == value1:
						foundOne = true
					case imglbl.Name == name2 && imglbl.Value == value2:
						foundTwo = true
					}
				}
				o.Expect(foundOne).To(o.BeTrue())
				o.Expect(foundTwo).To(o.BeTrue())
			})

			// this replaces coverage from the TestBuildOverrideLabels integration test
			g.It("Apply override image label configuration to build pod", func() {
				g.By("apply label cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				buildConfig.Spec.BuildOverrides.ImageLabels = []configv1.ImageLabel{
					{
						Name:  "VAR1",
						Value: "VALUE1",
					},
					{
						Name:  "VAR2",
						Value: "VALUE2",
					},
				}
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
				// there is no way programmatically to check leader election outcome
				g.By("waiting 10s for controller-manager leader election to complete")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildOverrides.ImageLabels).NotTo(o.BeNil())
				o.Expect(len(buildConfig.Spec.BuildOverrides.ImageLabels)).To(o.Equal(2))
				name1 := buildConfig.Spec.BuildOverrides.ImageLabels[0].Name
				value1 := buildConfig.Spec.BuildOverrides.ImageLabels[0].Value
				name2 := buildConfig.Spec.BuildOverrides.ImageLabels[1].Name
				value2 := buildConfig.Spec.BuildOverrides.ImageLabels[1].Value

				g.By("starting build simple-s2i-build and waiting for completion")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("checking build obj image label field")
				g.By("checking build stored in pod as well")
				// note, only the build stored in the Pod's "BUILD" env var has the updated proxy settings; they do not
				// get propagated to the associated build stored in etcd
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				build := getBuildFromPod(pod)
				o.Expect(build.Spec.Output.ImageLabels).NotTo(o.BeNil())
				foundOne := false
				foundTwo := false
				for _, imglbl := range build.Spec.Output.ImageLabels {
					switch {
					case imglbl.Name == name1 && imglbl.Value == value1:
						foundOne = true
					case imglbl.Name == name2 && imglbl.Value == value2:
						foundTwo = true
					}
				}
				o.Expect(foundOne).To(o.BeTrue())
				o.Expect(foundTwo).To(o.BeTrue())
			})

			// this replaces coverage from the TestBuildDefaultNodeSelectors integration test
			g.It("Apply node selector configuration to build pod", func() {
				g.By("apply node selector cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				selectors := map[string]string{"KEY": "VALUE", v1.LabelOSStable: "linux"}
				buildConfig.Spec.BuildOverrides.NodeSelector = selectors
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
				// there is no way programmatically to check leader election outcome
				g.By("waiting 10s for controller-manager leader election to complete")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildOverrides.NodeSelector).NotTo(o.BeNil())
				o.Expect(len(buildConfig.Spec.BuildOverrides.NodeSelector)).To(o.Equal(2))

				g.By("starting build simple-s2i-build and waiting for completion")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("checking build pod node selector")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Spec.NodeSelector).NotTo(o.BeNil())
				val, ok := pod.Spec.NodeSelector["KEY"]
				o.Expect(ok).To(o.BeTrue())
				o.Expect(val).To(o.Equal("VALUE"))
				val, ok = pod.Spec.NodeSelector[v1.LabelOSStable]
				o.Expect(ok).To(o.BeTrue())
				o.Expect(val).To(o.Equal("linux"))
				checkBuildPodUnschedulable(pod.Name)
			})

			// this replaces coverage from the TestBuildOverrideTolerations integration test
			g.It("Apply toleration override configuration to build pod", func() {
				g.By("apply toleration cluster configuration")
				buildConfig, err := oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				tolerations := []v1.Toleration{
					{
						Key:      "mykey1",
						Value:    "myvalue1",
						Effect:   v1.TaintEffectNoSchedule,
						Operator: v1.TolerationOpEqual,
					},
					{
						Key:      "mykey2",
						Value:    "myvalue2",
						Effect:   v1.TaintEffectNoSchedule,
						Operator: v1.TolerationOpEqual,
					},
				}

				buildConfig.Spec.BuildOverrides.Tolerations = tolerations
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Update(buildConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				checkOCMProgressing(operatorv1.ConditionTrue)
				checkOCMProgressing(operatorv1.ConditionFalse)
				checkDSRolloutState(true)
				checkDSRolloutState(false)
				// there is no way programmatically to check leader election outcome
				g.By("waiting 10s for controller-manager leader election to complete")
				time.Sleep(10 * time.Second)
				g.By("verify build.config is set")
				buildConfig, err = oc.AdminConfigClient().ConfigV1().Builds().Get("cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildConfig.Spec.BuildOverrides.Tolerations).NotTo(o.BeNil())
				o.Expect(len(buildConfig.Spec.BuildOverrides.Tolerations)).To(o.Equal(2))

				g.By("starting build simple-s2i-build and waiting for completion")
				br, err := exutil.StartBuildAndWait(oc, "simple-s2i-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("checking build pod tolerations")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(br.BuildName+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Spec.Tolerations).NotTo(o.BeNil())
				foundOne := false
				foundTwo := false
				for _, toleration := range pod.Spec.Tolerations {
					switch {
					case toleration.Key == "mykey1" && toleration.Value == "myvalue1" && toleration.Effect == v1.TaintEffectNoSchedule && toleration.Operator == v1.TolerationOpEqual:
						foundOne = true
					case toleration.Key == "mykey2" && toleration.Value == "myvalue2" && toleration.Effect == v1.TaintEffectNoSchedule && toleration.Operator == v1.TolerationOpEqual:
						foundTwo = true
					}
				}
				o.Expect(foundOne).To(o.BeTrue())
				o.Expect(foundTwo).To(o.BeTrue())
			})

		})
	})
})
