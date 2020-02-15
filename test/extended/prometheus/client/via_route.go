package client

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/transport"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/origin/test/extended/util"
)

// NewE2EPrometheusRouterClient returns a Prometheus HTTP API client configured to
// use the Prometheus route host, a bearer token, and no certificate verification.
func NewE2EPrometheusRouterClient(oc *util.CLI) (prometheusv1.API, error) {
	kubeClient := oc.AdminKubeClient()
	routeClient := oc.AdminRouteClient()

	// wait for prometheus service to exist
	err := wait.PollImmediate(time.Minute, time.Second, func() (bool, error) {
		_, err := kubeClient.CoreV1().Services("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
		return err == nil, nil
	})
	if err != nil {
		return nil, err
	}

	// wait for the prometheus route to exist
	var route *routev1.Route
	err = wait.PollImmediate(time.Minute, time.Second, func() (bool, error) {
		route, err = routeClient.RouteV1().Routes("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
		return err == nil, nil
	})
	if err != nil {
		return nil, err
	}

	// retrieve an openshift-monitoring service account secret
	var secret *corev1.Secret
	secrets, err := kubeClient.CoreV1().Secrets("openshift-monitoring").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, currSecret := range secrets.Items {
		if currSecret.Type == corev1.SecretTypeServiceAccountToken && strings.HasPrefix(currSecret.Name, "prometheus-") {
			secret = &currSecret
			break
		}
	}
	if secret == nil {
		return nil, fmt.Errorf("unable to locate service prometheus service account secret")
	}

	// prometheus API client, configured for route host and bearer token auth, and no cert verification
	client, err := api.NewClient(api.Config{
		Address: "https://" + route.Status.Ingress[0].Host,
		RoundTripper: transport.NewBearerAuthRoundTripper(
			string(secret.Data[corev1.ServiceAccountTokenKey]),
			&http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			},
		),
	})
	if err != nil {
		return nil, err
	}

	// return prometheus API
	return prometheusv1.NewAPI(client), nil
}
