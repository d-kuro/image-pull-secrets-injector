/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	"github.com/d-kuro/image-pull-secrets-injector/hook"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr     string
		domain          string
		secretName      string
		secretNamespace string
		certDir         string
	)

	var opts zap.Options
	opts.BindFlags(flag.CommandLine)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&domain, "domain", "docker.io",
		"The domain name of the image registry into which image-pull-secrets should be injected.")
	flag.StringVar(&secretName, "secret-name", "", "The Secret name to use for image-pull-secret.")
	flag.StringVar(&secretNamespace, "secret-namespace", "default", "The namespace where image-pull-secret exists.")
	flag.StringVar(&certDir, "cert-dir", "",
		"cert-dir is the directory that contains the server key and certificate. "+
			"If not set, webhook server would look up the server key and certificate in "+
			"{TempDir}/k8s-webhook-server/serving-certs. "+
			"The server key and certificate must be named tls.key and tls.crt, respectively.")

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     false,
		CertDir:            certDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/pod/mutate", &webhook.Admission{Handler: &hook.PodMutator{
		Log:             ctrl.Log.WithName("image-pull-secrets-injector"),
		Domain:          domain,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}})

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
