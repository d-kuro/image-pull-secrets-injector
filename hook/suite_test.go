package hook

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/d-kuro/image-pull-secrets-injector/hook/internal/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	defaultTestTimeout         = time.Second * 10
	defaultTestPollingInterval = time.Second * 1
	defaultImagePullSecretName = "image-pull-secret" // nolint:gosec
	defaultTestNamespace       = "default"
	podMutatingWebhookPath     = "/pod/mutate"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")

	webhookOptions := newWebhookInstallOptions()
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		WebhookInstallOptions: webhookOptions,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme,
		Host:    testEnv.WebhookInstallOptions.LocalServingHost,
		Port:    testEnv.WebhookInstallOptions.LocalServingPort,
		CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
	})
	Expect(err).ToNot(HaveOccurred())

	hookServer := mgr.GetWebhookServer()
	hookServer.Register(podMutatingWebhookPath, &webhook.Admission{Handler: &PodMutator{
		Log:             ctrl.Log.WithName("image-pull-secrets-injector"),
		Domain:          "docker.io",
		SecretName:      defaultImagePullSecretName,
		SecretNamespace: defaultTestNamespace,
	}})

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		fmt.Println(err)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = mgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		serverURL := fmt.Sprintf("%s:%d", testEnv.WebhookInstallOptions.LocalServingHost, testEnv.WebhookInstallOptions.LocalServingPort)
		conn, err := tls.DialWithDialer(d, "tcp", serverURL, &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec
		})
		if err != nil {
			return err
		}

		conn.Close()

		return nil
	}).Should(Succeed())

	// create image-pull-secret
	secret := newSecret(defaultImagePullSecretName)
	err = k8sClient.Create(context.Background(), secret)
	Expect(err).Should(Succeed())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func newWebhookInstallOptions() envtest.WebhookInstallOptions {
	failPolicy := admissionregistrationv1.Ignore
	sideEffect := admissionregistrationv1.SideEffectClassNoneOnDryRun

	return envtest.WebhookInstallOptions{
		MutatingWebhooks: []runtime.Object{
			&admissionregistrationv1.MutatingWebhookConfiguration{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "admissionregistration.k8s.io/v1",
					Kind:       "MutatingWebhookConfiguration",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "image-pull-secretes-mutator",
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name:          "pod-mutate-hook.d-kuro.github.com",
						FailurePolicy: &failPolicy,
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							Service: &admissionregistrationv1.ServiceReference{
								Path: testutil.ToPointerString(podMutatingWebhookPath),
							},
						},
						Rules: []admissionregistrationv1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1.OperationType{
									admissionregistrationv1.Create,
								},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
						SideEffects:             &sideEffect,
						AdmissionReviewVersions: []string{"v1beta1"},
					},
				},
			},
		},
	}
}
