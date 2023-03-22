package controlplane

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"

	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/go-logr/logr"

	"github.com/fi-ts/gardener-extension-authn/pkg/apis/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger, controllerConfig config.ControllerConfiguration) genericmutator.Ensurer {
	return &ensurer{
		logger:           logger.WithName("fits-authn-controlplane-ensurer"),
		controllerConfig: controllerConfig,
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client           client.Client
	logger           logr.Logger
	controllerConfig config.ControllerConfiguration
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, gctx gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec
	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, e.controllerConfig)
		ensureVolumeMounts(c, e.controllerConfig)
		ensureVolumes(ps, e.controllerConfig)
	}

	return nil
}

var (
	// config mount for authn-webhook-config that is specified at kube-apiserver commandline
	authnWebhookConfigVolumeMount = corev1.VolumeMount{
		Name:      "authn-webhook-config",
		MountPath: "/etc/webhook/config",
		ReadOnly:  true,
	}
	authnWebhookConfigVolume = corev1.Volume{
		Name: "authn-webhook-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "authn-webhook-config"},
			},
		},
	}
	// cert mount "kube-jwt-authn-webhook-server" that is referenced from the authn-webhook-config
	authnWebhookCertVolumeMount = corev1.VolumeMount{
		Name:      "authn-webhook-cert",
		MountPath: "/etc/webhook/certs",
		ReadOnly:  true,
	}
	authnWebhookCertVolume = corev1.Volume{
		Name: "authn-webhook-cert",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: "kube-jwt-authn-webhook-server",
			},
		},
	}
)

func ensureVolumeMounts(c *corev1.Container, controllerConfig config.ControllerConfiguration) {
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, authnWebhookConfigVolumeMount)
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, authnWebhookCertVolumeMount)
}

func ensureVolumes(ps *corev1.PodSpec, controllerConfig config.ControllerConfiguration) {
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, authnWebhookConfigVolume)
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, authnWebhookCertVolume)

}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, controllerConfig config.ControllerConfiguration) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--authentication-token-webhook-config-file=", "/etc/webhook/config/authn-webhook-config.json")
}
