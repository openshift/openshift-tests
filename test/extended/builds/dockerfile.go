package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/library-go/pkg/image/imageutil"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] build can have Dockerfile input", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-dockerfile-env", exutil.KubeConfigPath())
		dockerfileAdd  = exutil.FixturePath("testdata", "builds", "docker-add")
		testDockerfile = `
FROM library/busybox
USER 1001
`
		testDockerfile2 = `
FROM centos:7
USER 1001
`
		testDockerfile3 = `
FROM scratch
USER 1001
`
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("being created from new-build", func() {
			g.It("should create a image via new-build", func() {
				g.By("calling oc new-build with Dockerfile")
				err := oc.Run("new-build").Args("-D", "-", "--to", "busybox:custom").InputString(testDockerfile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("checking the buildconfig content")
				bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get("busybox", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(bc.Spec.Source.Git).To(o.BeNil())
				o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile))

				buildName := "busybox-1"
				g.By("expecting the Dockerfile build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName, nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("busybox", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("getting the build container image reference from ImageStream")
				image, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("busybox:custom", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				err = imageutil.ImageWithMetadata(&image.Image)
				o.Expect(err).NotTo(o.HaveOccurred())
				imageutil.ImageWithMetadataOrDie(&image.Image)
				o.Expect(image.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.User).To(o.Equal("1001"))
			})

			g.It("should create a image via new-build and infer the origin tag", func() {
				g.By("calling oc new-build with Dockerfile that uses the same tag as the output")
				err := oc.Run("new-build").Args("-D", "-").InputString(testDockerfile2).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("checking the buildconfig content")
				bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get("centos", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(bc.Spec.Source.Git).To(o.BeNil())
				o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile2))
				o.Expect(bc.Spec.Output.To).ToNot(o.BeNil())
				o.Expect(bc.Spec.Output.To.Name).To(o.Equal("centos:latest"))

				buildName := "centos-1"
				g.By("expecting the Dockerfile build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName, nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("centos", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("getting the built container image reference from ImageStream")
				image, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("centos:latest", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				err = imageutil.ImageWithMetadata(&image.Image)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(image.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.User).To(o.Equal("1001"))

				g.By("checking for the imported tag")
				_, err = oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get("centos:7", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("should be able to start a build from Dockerfile with FROM reference to scratch", func() {
				g.By("calling oc new-build with Dockerfile that uses scratch")
				err := oc.Run("new-build").Args("-D", "-").InputString(testDockerfile3).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("checking the buildconfig content")
				bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get("scratch", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile3))

				buildName := "scratch-1"
				g.By("expecting the Dockerfile build is in Complete phase")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName, nil, nil, nil)
				//debug for failures
				if err != nil {
					exutil.DumpBuildLogs("scratch", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("testing build image with invalid dockerfile content", func() {
				// https://bugzilla.redhat.com/show_bug.cgi?id=1694867
				g.By("creating a build")
				err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=centos").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, err := exutil.StartBuildAndWait(oc, "centos", fmt.Sprintf("--from-dir=%s", dockerfileAdd))
				br.AssertFailure()
				g.By("build log should have error about no file or directory")
				logs, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(logs).To(o.ContainSubstring("no such file or directory"))
			})

		})
	})
})
