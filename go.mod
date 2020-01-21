module github.com/openshift/extended-platform-tests

go 1.13

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/RangelReale/osincli v0.0.0-20160924135400-fababb0555f2
	github.com/apparentlymart/go-cidr v1.0.1
	github.com/aws/aws-sdk-go v1.16.26
	github.com/certifi/gocertifi v0.0.0-20190905060710-a5e0173ced67 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fsouza/go-dockerclient v0.0.0-20171004212419-da3951ba2e9e
	github.com/getsentry/raven-go v0.2.0 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/google/cadvisor v0.35.1-0.20200116204205-4abc562912f0 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jteeuwen/go-bindata v3.0.8-0.20151023091102-a0ff2567cfb7+incompatible
	github.com/lestrrat-go/jspointer v0.0.0-20181205001929-82fadba7561c // indirect
	github.com/lestrrat-go/jsref v0.0.0-20181205001954-1b590508f37d // indirect
	github.com/lestrrat-go/jsschema v0.0.0-20181205002244-5c81c58ffcc3 // indirect
	github.com/lestrrat-go/jsval v0.0.0-20181205002323-20277e9befc0 // indirect
	github.com/lestrrat-go/pdebug v0.0.0-20180220043849-39f9a71bcabe // indirect
	github.com/lestrrat-go/structinfo v0.0.0-20190212233437-acd51874663b // indirect
	github.com/lestrrat/go-jsschema v0.0.0-20181205002244-5c81c58ffcc3
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/runc v1.0.0-rc9.0.20200116150719-f6fb7a0338c3 // indirect
	github.com/openshift/api v3.9.1-0.20191201231411-9f834e337466+incompatible
	github.com/openshift/apiserver-library-go v0.0.0-20200117193640-5b4a83e36885
	github.com/openshift/client-go v0.0.0-20200116152001-92a2713fa240
	github.com/openshift/library-go v0.0.0-20200120153246-906409ae5e38
	github.com/pborman/uuid v1.2.0
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.3.2
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.4.0
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20191209160850-c0dbc17a3553
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.17.1
	k8s.io/apiextensions-apiserver v0.17.1
	k8s.io/apimachinery v0.17.1
	k8s.io/apiserver v0.17.1
	k8s.io/cli-runtime v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/component-base v0.17.1
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubectl v0.0.0
	k8s.io/kubernetes v1.17.1
	k8s.io/legacy-cloud-providers v0.0.0
	sigs.k8s.io/yaml v1.1.0
)

replace (
	bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
	github.com/golang/glog => github.com/openshift/golang-glog v0.0.0-20190322123450-3c92600d7533
	github.com/google/cadvisor => github.com/openshift/google-cadvisor v0.33.2-0.20190902063809-4db825a8ad0d
	github.com/jteeuwen/go-bindata => github.com/jteeuwen/go-bindata v3.0.8-0.20151023091102-a0ff2567cfb7+incompatible
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v1.4.1-0.20190902091932-d0603c19fe78
	github.com/opencontainers/runc => github.com/openshift/opencontainers-runc v1.0.0-rc4.0.20190926164333-b942ff4cc6f8
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200117162508-e7ccdda6ba67
	k8s.io/api => k8s.io/api v0.17.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.1
	k8s.io/apimachinery => github.com/openshift/kubernetes-apimachinery v0.0.0-20191121175448-79c2a76c473a
	k8s.io/apiserver => github.com/openshift/kubernetes-apiserver v0.0.0-20200109101329-ed563d1b80a1
	k8s.io/cli-runtime => github.com/openshift/kubernetes-cli-runtime v0.0.0-20200115000600-01f2488fd0b7
	k8s.io/client-go => github.com/openshift/kubernetes-client-go v0.0.0-20200106170045-1fda2942f64d
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.1
	k8s.io/code-generator => k8s.io/code-generator v0.17.1
	k8s.io/component-base => k8s.io/component-base v0.17.1
	k8s.io/cri-api => k8s.io/cri-api v0.17.1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.1
	k8s.io/kube-aggregator => github.com/openshift/kubernetes-kube-aggregator v0.0.0-20191209133208-1e3c0eec4d61
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.1
	k8s.io/kubectl => k8s.io/kubectl v0.17.1
	k8s.io/kubelet => k8s.io/kubelet v0.17.1

	k8s.io/kubernetes => github.com/openshift/kubernetes v1.17.0-alpha.0.0.20200120180958-5945c3b07163
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191121182806-cdbd52110e91
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191121181631-c7d4ee0ffc0e
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191121181040-36c9528858d2
)
