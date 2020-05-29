package cli

import (
        "encoding/json"
        "fmt"
        "log"
        "os"
        "regexp"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[sig-cli] Workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc                     = exutil.NewCLI("oc", exutil.KubeConfigPath())
	)

	g.It("Critical-30285-Checking oc version before login [Serial]", func() {
		g.By("check version info with default KUBECONFIG") 
		out, err := oc.Run("version").Args("-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
                versionInfo := &VersionInfo{}
                if err := json.Unmarshal([]byte(out), &versionInfo); err != nil {
                        log.Fatal("unable to decode version with error: %v", err)
                }
                reg := regexp.MustCompile(`(openshift-clients-)?[\d]+\.[\d]+\.[\d]+\-[\d]{12}(\-[\w]+)?`)
                if match := reg.MatchString(versionInfo.ClientInfo.GitVersion); !match {
                        log.Fatal("varification version with error: %v", err) 
                }         

		g.By("check version info without --kubeconfig,~/.kube/config,KUBECONFIG") 
                kubeconfig :=exutil.KubeConfigPath()
                defer os.Setenv("KUBECONFIG", kubeconfig)
                os.Setenv("KUBECONFIG", "")
                defaultHomeDir,_ := os.UserHomeDir()
                defaultConfig := defaultHomeDir + "/.kube/config"
		if _, err := os.Stat(defaultConfig); err == nil {
                        defer os.Rename(defaultHomeDir+"/.kube/configback",defaultHomeDir+"/.kube/config")
			fmt.Printf("found default config file %s, now rename it\n", defaultConfig)
    			os.Rename(defaultConfig, defaultHomeDir+"/.kube/configback")
		}
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

func closeFile(f *os.File) {
    fmt.Println("closing")
    err := f.Close()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}

type ClientVersion struct {
        BuildDate     string                              `json:"buildDate"`
        Compiler      string                              `json:"compiler"`
        GitCommit     string                              `json:"gitCommit"`
        GitTreeState  string                              `json:"gitTreeState"`
        GitVersion    string                              `json:"gitVersion"`
        GoVersion     string                              `json:"goVersion"`
        Major         string                              `json:"major"`
        Minor         string                              `json:"minor"`
        Platform      string                              `json:"platform"`
}

type ServerVersion struct {
        BuildDate     string                              `json:"buildDate"`
        Compiler      string                              `json:"compiler"`
        GitCommit     string                              `json:"gitCommit"`
        GitTreeState  string                              `json:"gitTreeState"`
        GitVersion    string                              `json:"gitVersion"`
        GoVersion     string                              `json:"goVersion"`
        Major         string                              `json:"major"`
        Minor         string                              `json:"minor"`
        Platform      string                              `json:"platform"`
}


type VersionInfo struct {
       ClientInfo     ClientVersion                       `json:"ClientVersion"`
       OpenshiftVersion  string                           `json:"openshiftVersion"` 
       ServerInfo     ServerVersion                       `json:"ServerVersion"`
}
