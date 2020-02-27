package images

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Feature:Image] oc tag", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("image-oc-tag", exutil.KubeConfigPath())

	g.It("should preserve image reference for external images", func() {
		const (
			externalRepository = "busybox"
			externalImage      = "busybox:latest"
			isName             = "busybox"
			isName2            = "busybox2"
		)

		g.By("import an external image")

		err := oc.Run("tag").Args("--source=docker", externalImage, isName+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the created image stream references the external registry
		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(isName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag1 := is.Status.Tags[0]
		o.Expect(tag1.Tag).To(o.Equal("latest"))
		o.Expect(tag1.Items).To(o.HaveLen(1))
		o.Expect(tag1.Items[0].DockerImageReference).To(o.HavePrefix(externalRepository + "@"))

		g.By("copy the image to another image stream")

		err = oc.Run("tag").Args("--source=istag", isName+":latest", isName2+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName2, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the new image stream references the still uses the external registry
		is, err = oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(isName2, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag2 := is.Status.Tags[0]
		o.Expect(tag2.Tag).To(o.Equal("latest"))
		o.Expect(tag2.Items).To(o.HaveLen(1))
		o.Expect(tag2.Items[0].DockerImageReference).To(o.Equal(tag1.Items[0].DockerImageReference))
	})

	g.It("should change image reference for internal images", func() {
		const (
			isName     = "localimage"
			isName2    = "localimage2"
			dockerfile = `FROM busybox:latest
RUN touch /test-image
`
		)

		g.By("determine the name of the integrated registry")

		registryHost, err := oc.Run("registry").Args("info").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("build an image")

		err = oc.Run("new-build").Args("-D", "-", "--to", isName+":latest").InputString(dockerfile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), isName+"-1", nil, nil, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the created image stream references the integrated registry
		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(isName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag := is.Status.Tags[0]
		o.Expect(tag.Tag).To(o.Equal("latest"))
		o.Expect(tag.Items).To(o.HaveLen(1))
		o.Expect(tag.Items[0].DockerImageReference).To(o.HavePrefix(fmt.Sprintf("%s/%s/%s@", registryHost, oc.Namespace(), isName)))

		// extract the image digest
		ref := tag.Items[0].DockerImageReference
		digest := ref[strings.Index(ref, "@")+1:]
		o.Expect(digest).To(o.HavePrefix("sha256:"))

		g.By("copy the image to another image stream")

		err = oc.Run("tag").Args("--source=istag", isName+":latest", isName2+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName2, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the new image stream uses its own name in the image reference
		is, err = oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(isName2, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag = is.Status.Tags[0]
		o.Expect(tag.Tag).To(o.Equal("latest"))
		o.Expect(tag.Items).To(o.HaveLen(1))
		o.Expect(tag.Items[0].DockerImageReference).To(o.Equal(fmt.Sprintf("%s/%s/%s@%s", registryHost, oc.Namespace(), isName2, digest)))
	})
})
