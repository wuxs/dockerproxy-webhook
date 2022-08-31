package webhooks

import (
	"context"
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

var mutatingLog = ctrl.Log.WithName("mutating")

// +kubebuilder:webhook:path=/mutate-v1-deployment,mutating=true,failurePolicy=fail,groups="",resources=deployments,verbs=create;update,versions=v1,name=mdeployment.kb.io,admissionReviewVersions=v1,sideEffects=none

// Mutating annotates Pods
type Mutating struct {
	Client  client.Client
	decoder *admission.Decoder
}

const ConfigMapName = "dockerproxy"

// Mutating adds an annotation to every incoming pods.
func (a *Mutating) Handle(ctx context.Context, req admission.Request) admission.Response {
	mutatingLog.Info("request info", "req", req)
	if req.Kind.Kind != "Pod" {
		mutatingLog.Info("kind is not match, skip")
		return admission.Allowed("skip")
	}

	obj := &corev1.ConfigMap{}
	err := a.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: ConfigMapName}, obj)
	if err != nil {
		return admission.Allowed("skip")
	}

	pod := &corev1.Pod{}
	err = a.decoder.Decode(req, pod)
	if err != nil {
		mutatingLog.Error(err, "decode Pod error")
		return admission.Allowed("skip")
	}
	containers := make([]corev1.Container, 0)
	for _, container := range pod.Spec.Containers {
		items := strings.Split(container.Image, "/")
		if len(items) < 2 {
			continue
		}
		if len(items) == 2 {
			items = append([]string{"dockerproxy.com"}, items...)
		} else if items[0] == "docker.io" {
			items[0] = "dockerproxy.com"
		}
		container.Image = strings.Join(items, "/")
		containers = append(containers, container)
	}
	pod.Spec.Containers = containers

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		mutatingLog.Error(err, "marshal Deployment error")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	mutatingLog.Info("new pod info", "pod", pod)
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// Mutating implements admission.DecoderInjector.
// A decoder will be automatically injected.

// InjectDecoder injects the decoder.
func (a *Mutating) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}
