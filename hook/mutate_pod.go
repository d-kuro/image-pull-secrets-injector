package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ref: https://github.com/kubernetes-sigs/controller-tools/blob/v0.4.0/pkg/webhook/parser.go
// +kubebuilder:webhook:webhookVersions=v1,failurePolicy=ignore,matchPolicy=Equivalent,groups="",resources=pods,verbs=create,versions=v1,name=pod-mutate-hook.d-kuro.github.com,path=/pod/mutate,mutating=true,sideEffects=NoneOnDryRun
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create

// PodMutator annotates Pods.
type PodMutator struct {
	client  client.Client
	decoder *admission.Decoder

	Log logr.Logger

	Domain          string
	SecretName      string
	SecretNamespace string
}

// PodMutator adds an annotation to every incoming pods.
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := m.Log.
		WithValues("uid", req.UID).
		WithValues("pod", fmt.Sprintf("%s%c%s", req.Namespace, types.Separator, req.Name))

	var pod corev1.Pod

	err := m.decoder.Decode(req, &pod)
	if err != nil {
		log.Error(err, "unable to decode Pod")

		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := m.defaultingImagePullSecretes(ctx, &pod); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		log.Error(err, "unable to marshal Pod")

		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (m *PodMutator) defaultingImagePullSecretes(ctx context.Context, pod *corev1.Pod) error {
	if findImagePullSecrets(m.SecretName, pod) {
		return nil
	}

	if !matchContainerImageDomain(m.Domain, pod) {
		return nil
	}

	secret := corev1.Secret{}
	err := m.client.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: m.SecretName}, &secret)
	switch {
	case errors.IsNotFound(err):
		originalSecret := corev1.Secret{}
		if err := m.client.Get(ctx, types.NamespacedName{Namespace: m.SecretNamespace, Name: m.SecretName}, &originalSecret); err != nil {
			return err
		}

		newSecret := originalSecret.DeepCopy()
		newSecret.ObjectMeta.ResourceVersion = "" // resourceVersion should not be set on objects to be created
		newSecret.ObjectMeta.Namespace = pod.Namespace

		if err := m.client.Create(ctx, newSecret, &client.CreateOptions{}); err != nil {
			return err
		}

	case err != nil:
		return err
	}

	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: m.SecretName})

	return nil
}

func findImagePullSecrets(secretName string, pod *corev1.Pod) bool {
	for _, objRef := range pod.Spec.ImagePullSecrets {
		if secretName == objRef.Name {
			return true
		}
	}

	return false
}

func matchContainerImageDomain(domain string, pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		imageDomain, _ := splitDockerDomain(container.Image)
		if domain == imageDomain {
			return true
		}
	}

	return false
}

// PodMutator implements inject.Client.
// A client will be automatically injected.

// InjectClient injects the client.
func (m *PodMutator) InjectClient(c client.Client) error {
	m.client = c

	return nil
}

// PodMutator implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (m *PodMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d

	return nil
}
