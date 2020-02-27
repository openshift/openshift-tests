package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/util/sets"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/openshift-tests/test/extended/util"
)

var allCanRunPerms int32 = 0777

var blacklist = sets.NewString(
	"login.sh",    // fails because so much depends on `oc login`
	"migrate.sh",  // seems unnecessary since we never run it
	"newapp.sh",   // our image is missing git, so a lot of it doesn't work
	"policy.sh",   // fails because so much depends on `oc login`
	"registry.sh", // this was dead before we made this switch
)

var _ = g.Describe("[Suite:openshift/test-cmd][Serial][Disruptive] test-cmd:", func() {
	hacklibDir := exutil.FixturePath("testdata", "cmd", "hack")
	testsDir := exutil.FixturePath("testdata", "cmd", "test", "cmd")

	oc := exutil.NewCLI("test-cmd", exutil.KubeConfigPath())

	cmData, _ := getDirDataAndKeyPathMap(testsDir)
	for _, filename := range sets.StringKeySet(cmData).List() {
		// only make tests for the bash files
		if !strings.HasSuffix(filename, ".sh") {
			continue
		}
		currFilename := filename
		if blacklist.Has(currFilename) {
			continue
		}

		g.It("test/cmd/"+currFilename, func() {
			oc.AddExplicitResourceToDelete(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}, "", "cmd-"+currFilename[0:len(currFilename)-3])

			hacklibVolume, hacklibVolumeMount := createConfigMapForDir(oc, hacklibDir, "/var/tests/hack")
			testsVolume, testsVolumeMount := createConfigMapForDir(oc, testsDir, "/var/tests/test/cmd")

			kubeconfigCont, _, err := oc.AsAdmin().Run("config").Args("view", "--flatten", "--minify").Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kubeconfig",
				},
				Data: map[string]string{"kubeconfig": kubeconfigCont},
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			cliIS, err := oc.AdminImageClient().ImageV1().ImageStreams("openshift").Get("cli", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var cliImageRef string
			for _, tag := range cliIS.Status.Tags {
				if tag.Tag == "latest" {
					cliImageRef = tag.Items[0].DockerImageReference
					break
				}
			}

			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get("cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			log, errs := exutil.RunOneShotCommandPod(oc, "test-cmd", cliImageRef, "/var/tests/test/cmd/"+currFilename,
				[]corev1.VolumeMount{
					*hacklibVolumeMount,
					*testsVolumeMount,
					{
						Name:      "kubeconfig",
						MountPath: "/var/tests/kubeconfig",
					},
				},
				[]corev1.Volume{
					*hacklibVolume,
					*testsVolume,
					{
						Name: "kubeconfig",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "kubeconfig",
								},
							},
						},
					},
				},
				[]corev1.EnvVar{
					{Name: "KUBECONFIG_TESTS", Value: "/var/tests/kubeconfig/kubeconfig"},
					{Name: "KUBERNETES_MASTER", Value: infra.Status.APIServerURL},
					{Name: "USER_TOKEN", Value: oc.UserConfig().BearerToken},
					{Name: "TESTS_DIR", Value: "/var/tests/test/cmd"},
					{Name: "TEST_NAME", Value: currFilename[0 : len(currFilename)-3]},
					{Name: "TEST_DATA", Value: "/var/tests/test/cmd/testdata"},
				},
				5*time.Minute,
			)
			e2e.Logf("Logs from the container: %s", log)
			o.Expect(errs).To(o.HaveLen(0))
		})
	}
})

func createConfigMapForDir(oc *exutil.CLI, dirname, mountDirname string) (*corev1.Volume, *corev1.VolumeMount) {
	cmData, keyMapping := getDirDataAndKeyPathMap(dirname)

	cmName := strings.ReplaceAll(strings.SplitAfter(dirname, filepath.Join("testdata", "cmd"))[1], "/", "-")[1:]
	_, err := oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cmName,
		},
		Data: cmData,
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	volume := &corev1.Volume{
		Name: cmName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				DefaultMode:          &allCanRunPerms,
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				Items:                keyMapping,
			},
		},
	}
	volumeMount := &corev1.VolumeMount{
		Name:      cmName,
		MountPath: mountDirname,
	}

	return volume, volumeMount
}

func getDirDataAndKeyPathMap(dir string) (map[string]string, []corev1.KeyToPath) {
	configMapData := map[string]string{}
	keyPathMapping := []corev1.KeyToPath{}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if os.IsPermission(err) {
			e2e.Logf("no permissions to access '%s', skipping: %v", err)
		}

		// skip reading dirs
		if info.IsDir() {
			return nil
		}
		fileCont, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		// _, fileName := filepath.Split(path)
		mountedPath := strings.SplitAfter(path, fmt.Sprintf("%s/", dir))[1]

		key := strings.ReplaceAll(mountedPath, "/", "-")
		configMapData[key] = string(fileCont)
		keyPathMapping = append(keyPathMapping, corev1.KeyToPath{Key: key, Path: mountedPath})

		return nil
	})
	if err != nil {
		panic(err)
	}

	return configMapData, keyPathMapping
}
