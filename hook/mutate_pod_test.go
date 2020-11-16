package hook

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = ginkgo.Describe("image-pull-secrets mutating webhook", func() {
	ginkgo.Context("when creating Pod", func() {
		ginkgo.It("should set imagePullSecrets", func() {
			ctx := context.Background()

			pod := newPod("docker-io-image")
			err := k8sClient.Create(ctx, pod)
			gomega.Expect(err).Should(gomega.Succeed())

			var createdPod corev1.Pod
			gomega.Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &createdPod); err != nil {
					return err
				}

				if pod.Spec.ImagePullSecrets == nil || len(pod.Spec.ImagePullSecrets) == 0 {
					return fmt.Errorf("imagePullSecrets not found")
				}

				matched := false
				for _, objRef := range pod.Spec.ImagePullSecrets {
					if objRef.Name == defaultImagePullSecretName {
						matched = true
					}
				}

				if !matched {
					return fmt.Errorf("image pull secrets are not registered")
				}

				return nil
			}, /*timeout*/ defaultTestTimeout /*pollingInterval*/, defaultTestPollingInterval).Should(gomega.Succeed())
		})

		ginkgo.It("should set imagePullSecrets for different namespace", func() {
			const testNamespace = "integration-test"
			ctx := context.Background()

			namespace := newNamespace(testNamespace)
			err := k8sClient.Create(ctx, namespace)
			gomega.Expect(err).Should(gomega.Succeed())

			pod := newPod("docker-io-image-ns", withPodNamespace(testNamespace))
			err = k8sClient.Create(ctx, pod)
			gomega.Expect(err).Should(gomega.Succeed())

			var createdPod corev1.Pod
			var createdSecret corev1.Secret
			gomega.Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultImagePullSecretName, Namespace: pod.Namespace}, &createdSecret); err != nil {
					return err
				}

				if err := k8sClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &createdPod); err != nil {
					return err
				}

				if pod.Spec.ImagePullSecrets == nil || len(pod.Spec.ImagePullSecrets) == 0 {
					return fmt.Errorf("imagePullSecrets not found")
				}

				matched := false
				for _, objRef := range pod.Spec.ImagePullSecrets {
					if objRef.Name == defaultImagePullSecretName {
						matched = true
					}
				}

				if !matched {
					return fmt.Errorf("image pull secrets are not registered")
				}

				return nil
			}, /*timeout*/ defaultTestTimeout /*pollingInterval*/, defaultTestPollingInterval).Should(gomega.Succeed())
		})

		ginkgo.It("should not set imagePullSecrets for unmatched domain", func() {
			ctx := context.Background()

			pod := newPod("k8s-gcr-io-image", withPodImage("k8s.gcr.io/autoscaling/cluster-autoscaler:v1.17.4"))
			err := k8sClient.Create(ctx, pod)
			gomega.Expect(err).Should(gomega.Succeed())

			var createdPod corev1.Pod
			gomega.Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &createdPod); err != nil {
					return err
				}

				if pod.Spec.ImagePullSecrets != nil || len(pod.Spec.ImagePullSecrets) != 0 {
					return fmt.Errorf("imagePullSecrets was injected for an unmatched domain")
				}

				return nil
			}, /*timeout*/ defaultTestTimeout /*pollingInterval*/, defaultTestPollingInterval).Should(gomega.Succeed())
		})
	})
})

func newPod(name string, options ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultTestNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
				},
			},
		},
	}

	for _, option := range options {
		option(pod)
	}

	return pod
}

func withPodImage(image string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Spec.Containers[0].Image = image
	}
}

func withPodNamespace(namespace string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.ObjectMeta.Namespace = namespace
	}
}

func newSecret(name string, options ...func(*corev1.Secret)) *corev1.Secret {
	data := make(map[string][]byte, 1)
	data[corev1.DockerConfigJsonKey] = []byte(
		`{"auths":{"https://index.docker.io/v1/":{"username":"d-kuro","password":"pass","auth":"ZC1rdXJvOnBhc3M="}}}`)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultTestNamespace,
		},
		Data: data,
		Type: corev1.SecretTypeDockerConfigJson,
	}

	for _, option := range options {
		option(secret)
	}

	return secret
}

func newNamespace(name string, options ...func(*corev1.Namespace)) *corev1.Namespace {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, option := range options {
		option(namespace)
	}

	return namespace
}
