package kapiserver

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"

	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("fits-authn-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, _ gcontext.GardenContext, new, _ *appsv1.Deployment) error {
	namespace := new.Namespace

	kubeconfig, err := webhookKubeconfig(namespace)
	if err != nil {
		return err
	}

	e.logger.Info("ensuring webhook configmap")

	e.logger.Info(namespace)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "authn-webhook-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"authn-webhook-config.json": string(kubeconfig),
		},
	}

	err = e.client.Get(ctx, client.ObjectKeyFromObject(cm), cm)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		err := e.client.Create(ctx, cm)
		if err != nil {
			return err
		}
	} else {
		err := e.client.Update(ctx, cm)
		if err != nil {
			return err
		}
	}

	template := &new.Spec.Template
	ps := &template.Spec
	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		e.logger.Info("ensuring kube-apiserver deployment")
		ensureKubeAPIServerCommandLineArgs(c)
		ensureVolumeMounts(c)
		ensureVolumes(ps)
	}

	return nil
}

func webhookKubeconfig(namespace string) ([]byte, error) {
	var (
		contextName = "kube-jwt-authn-webhook"
		url         = fmt.Sprintf("https://kube-jwt-authn-webhook.%s.svc.cluster.local/authenticate", namespace)
	)

	config := &configv1.Config{
		CurrentContext: contextName,
		Clusters: []configv1.NamedCluster{
			{
				Name: contextName,
				Cluster: configv1.Cluster{
					Server: url,
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: contextName,
				Context: configv1.Context{
					Cluster:  contextName,
					AuthInfo: contextName,
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name:     contextName,
				AuthInfo: configv1.AuthInfo{},
			},
		},
	}

	kubeconfig, err := runtime.Encode(configlatest.Codec, config)
	if err != nil {
		return nil, fmt.Errorf("unable to encode webhook kubeconfig: %w", err)
	}

	return kubeconfig, nil
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

func ensureVolumeMounts(c *corev1.Container) {
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, authnWebhookConfigVolumeMount)
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, authnWebhookCertVolumeMount)
}

func ensureVolumes(ps *corev1.PodSpec) {
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, authnWebhookConfigVolume)
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, authnWebhookCertVolume)

}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--authentication-token-webhook-config-file=", "/etc/webhook/config/authn-webhook-config.json")
}
