package oauth

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	testutil "github.com/openshift/openshift-tests/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/oauth] LDAP", func() {
	defer g.GinkgoRecover()
	var (
		oc = testutil.NewCLI("oauth-ldap", testutil.KubeConfigPath())
	)

	g.It("should start an OpenLDAP test server", func() {
		_, _, err := testutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
