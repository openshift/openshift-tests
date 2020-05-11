package cli

import (
        "fmt"
        "log"
        "os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[cli]oc version[Conformance]", func() {
	defer g.GinkgoRecover()

	var (
		oc                     = exutil.NewCLI("oc-rsh", exutil.KubeConfigPath())
	)

	g.Describe("oc version before login", func() {
		g.It("check version without login", func() {
			g.By("check version info with default KUBECONFIG") 
			out, err := oc.Run("version").Args("-o", "yaml").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("serverVersion"))

			g.By("check version info without --kubeconfig,~/.kube/config,KUBECONFIG") 
                        kubeconfig :=exutil.KubeConfigPath()
                        defer os.Setenv("KUBECONFIG", kubeconfig)
                        os.Setenv("KUBECONFIG", "")
			out, err = oc.Run("version").Args("-o", "yaml").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("clientVersion"))
                        
			g.By("check version info with empty kubeconfig")
                        emptyFile, err := os.Create("/tmp/emptykubeconfig")
	                if err != nil {
		                log.Fatal(err)
	                } 
                        defer closeFile(emptyFile)
			out, err = oc.Run("version").Args("--kubeconfig", "/tmp/emptykubeconfig").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Client Version"))
                        
			g.By("check version info with correct kubeconfig")
			out, err = oc.Run("version").Args("--kubeconfig", kubeconfig).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Server Version"))
		})
	})
})

func closeFile(f *os.File) {
    fmt.Println("closing")
    err := f.Close()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
