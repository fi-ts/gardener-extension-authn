package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/v1alpha1"
	"github.com/fi-ts/gardener-extension-authn/pkg/apis/config"
	"github.com/fi-ts/gardener-extension-authn/pkg/imagevector"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardenercontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/go-logr/logr"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(config config.ControllerConfiguration) extension.Actuator {
	return &actuator{
		log:    log.Log.WithName("authn-controller"),
		config: config,
	}
}

type actuator struct {
	log     logr.Logger
	client  client.Client
	decoder runtime.Decoder
	config  config.ControllerConfiguration
}

// InjectClient injects the controller runtime client into the reconciler.
func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// InjectScheme injects the given scheme into the reconciler.
func (a *actuator) InjectScheme(scheme *runtime.Scheme) error {
	a.decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()
	return nil
}

// Reconcile the Extension resource.
func (a *actuator) Reconcile(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	if ex.Spec.ProviderConfig == nil {
		return nil
	}

	namespace := ex.GetNamespace()

	cluster, err := controller.GetCluster(ctx, a.client, namespace)
	if err != nil {
		return err
	}

	authnConfig := &v1alpha1.AuthnConfig{}
	if _, _, err := a.decoder.Decode(ex.Spec.ProviderConfig.Raw, nil, authnConfig); err != nil {
		return fmt.Errorf("failed to decode provider config: %w", err)
	}

	if err := a.createResources(ctx, authnConfig, cluster, namespace); err != nil {
		return err
	}

	return nil
}

// Delete the Extension resource.
func (a *actuator) Delete(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	return a.deleteResources(ctx, ex.GetNamespace())
}

// Restore the Extension resource.
func (a *actuator) Restore(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, ex)
}

// Migrate the Extension resource.
func (a *actuator) Migrate(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	return nil
}

func (a *actuator) createResources(ctx context.Context, authConfig *v1alpha1.AuthnConfig, cluster *controller.Cluster, namespace string) error {
	shootObjects := shootObjects()

	seedObjects, err := seedObjects(&a.config, authConfig, cluster, namespace)
	if err != nil {
		return err
	}

	shootResources, err := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).AddAllAndSerialize(shootObjects...)
	if err != nil {
		return err
	}

	seedResources, err := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer).AddAllAndSerialize(seedObjects...)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForShoot(ctx, a.client, namespace, v1alpha1.ShootAuthResourceName, false, shootResources); err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, a.client, namespace, v1alpha1.SeedAuthResourceName, false, seedResources); err != nil {
		return err
	}

	return nil
}

func (a *actuator) deleteResources(ctx context.Context, namespace string) error {
	a.log.Info("deleting managed resource for registry cache")

	if err := managedresources.Delete(ctx, a.client, namespace, v1alpha1.ShootAuthResourceName, false); err != nil {
		return err
	}

	if err := managedresources.Delete(ctx, a.client, namespace, v1alpha1.SeedAuthResourceName, false); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := managedresources.WaitUntilDeleted(timeoutCtx, a.client, namespace, v1alpha1.ShootAuthResourceName); err != nil {
		return err
	}

	if err := managedresources.WaitUntilDeleted(timeoutCtx, a.client, namespace, v1alpha1.SeedAuthResourceName); err != nil {
		return err
	}

	return nil
}

func seedObjects(cc *config.ControllerConfiguration, authConfig *v1alpha1.AuthnConfig, cluster *controller.Cluster, namespace string) ([]client.Object, error) {
	authnImage, err := imagevector.ImageVector().FindImage("authn-webhook")
	if err != nil {
		return nil, fmt.Errorf("failed to find authn-webhook image: %w", err)
	}
	grcImage, err := imagevector.ImageVector().FindImage("group-rolebinding-controller")
	if err != nil {
		return nil, fmt.Errorf("failed to find group-rolebinding-controller image: %w", err)
	}

	tenant, ok := cluster.Shoot.Annotations[tag.ClusterTenant]
	if !ok {
		return nil, fmt.Errorf("cluster has no tenant annotation")
	}

	replicas := int32(1)
	if gardenercontroller.IsHibernated(cluster) {
		replicas = 0
	}

	webhookDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-jwt-authn-webhook",
			Namespace: namespace,
			Labels: map[string]string{
				"k8s-app": "kube-jwt-authn-webhook",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Pointer(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-jwt-authn-webhook",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "kube-jwt-authn-webhook",
						"app":     "kube-jwt-authn-webhook",
						"networking.gardener.cloud/from-prometheus":    "allowed",
						"networking.gardener.cloud/to-dns":             "allowed",
						"networking.gardener.cloud/to-shoot-apiserver": "allowed",
						"networking.gardener.cloud/to-public-networks": "allowed",
					},
					Annotations: map[string]string{
						"scheduler.alpha.kubernetes.io/critical-pod": "",
						"prometheus.io/scrape":                       "true",
						"prometheus.io/path":                         "/metrics",
						"prometheus.io/port":                         "2112",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "kubernetes-authn-webhook",
							Image:           authnImage.Repository,
							ImagePullPolicy: corev1.PullAlways,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 443,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "monitoring",
									ContainerPort: 2112,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LISTEN",
									Value: ":443",
								},
								{
									Name:  "TLSCERTFILE",
									Value: "/etc/webhook/certs/kube-jwt-authn-webhook-server.crt",
								},
								{
									Name:  "TLSKEYFILE",
									Value: "/etc/webhook/certs/kube-jwt-authn-webhook-server.key",
								},
								{
									Name:  "ISSUER",
									Value: authConfig.Issuer,
								},
								{
									Name:  "CLIENTID",
									Value: authConfig.ClientID,
								},
								{
									Name:  "GROUPSPREFIXTOREMOVE",
									Value: "k8s",
								},
								{
									Name:  "TENANT",
									Value: tenant,
								},
								{
									Name:  "PROVIDERTENANT",
									Value: cc.Auth.ProviderTenant,
								},
								{
									Name:  "CLUSTER",
									Value: cluster.ObjectMeta.Name,
								},
								{
									Name: "METAL_URL",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kube-jwt-authn-webhook-metalapi-secret",
											},
											Key: "metalapi-url",
										},
									},
								},
								{
									Name: "METAL_HMAC",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kube-jwt-authn-webhook-metalapi-secret",
											},
											Key: "metalapi-hmac",
										},
									},
								},
								{
									Name: "METAL_HMACAUTHTYPE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kube-jwt-authn-webhook-metalapi-secret",
											},
											Key: "metalapi-authtype",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "webhook-certs",
									MountPath: "/etc/webhook/certs",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "webhook-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-jwt-authn-webhook-server",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
				},
			},
		},
	}

	grcDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "group-rolebinding-controller",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "group-rolebinding-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Pointer(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "group-rolebinding-controller",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "group-rolebinding-controller",
						"networking.gardener.cloud/from-prometheus":    "allowed",
						"networking.gardener.cloud/to-dns":             "allowed",
						"networking.gardener.cloud/to-shoot-apiserver": "allowed",
						"networking.gardener.cloud/to-public-networks": "allowed",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "group-rolebinding-controller",
							Image:           grcImage.Repository,
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"/group-rolebinding-controller"},
							Args: []string{
								"--excludeNamespaces=kube-system,kube-public,kube-node-lease,default",
								"--expectedGroupsList=admin,edit,view",
								fmt.Sprintf("--clustername=%s", cluster.Shoot.Name),
								"--kubeconfig=/var/lib/group-rolebinding-controller/kubeconfig",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "group-rolebinding-controller",
									MountPath: "/var/lib/group-rolebinding-controller",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "group-rolebinding-controller",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "group-rolebinding-controller",
								},
							},
						},
					},
				},
			},
		},
	}

	objects := []client.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-jwt-authn-webhook-metalapi-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"metalapi-url":      cc.Auth.MetalURL,
				"metalapi-hmac":     cc.Auth.MetalHMAC,
				"metalapi-authtype": cc.Auth.MetalAuthType,
			},
		},
		webhookDeployment,
		grcDeployment,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-jwt-authn-webhook",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "kube-jwt-authn-webhook",
				},
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": "kube-jwt-authn-webhook",
				},
				Ports: []corev1.ServicePort{
					{
						Port:       443,
						TargetPort: intstr.FromInt(443),
					},
				},
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-jwt-authn-webhook-allow-namespace",
				Namespace: namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "kube-jwt-authn-webhook",
					},
				},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app":  "kubernetes",
										"role": "apiserver",
									},
								},
							},
						},
					},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeapi2kube-jwt-authn-webhook",
				Namespace: namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "kube-jwt-authn-webhook",
					},
				},
				Egress: []networkingv1.NetworkPolicyEgressRule{
					{
						To: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app":  "kubernetes",
										"role": "apiserver",
									},
								},
							},
						},
					},
				},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			},
		},
	}

	if cc.ImagePullSecret != nil && cc.ImagePullSecret.DockerConfigJSON != "" {
		objects = append(objects, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-jwt-authn-webhook-registry-credentials",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "kube-jwt-authn-webhook-registry-credentials",
				},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			StringData: map[string]string{
				".dockerconfigjson": cc.ImagePullSecret.DockerConfigJSON,
			},
		}, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "group-rolebinding-controller-registry-credentials",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "group-rolebinding-controller-registry-credentials",
				},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			StringData: map[string]string{
				".dockerconfigjson": cc.ImagePullSecret.DockerConfigJSON,
			},
		})

		webhookDeployment.Spec.Template.Spec.ImagePullSecrets = append(webhookDeployment.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{
			Name: "kube-jwt-authn-webhook-registry-credentials",
		})
		grcDeployment.Spec.Template.Spec.ImagePullSecrets = append(grcDeployment.Spec.Template.Spec.ImagePullSecrets, corev1.LocalObjectReference{
			Name: "group-rolebinding-controller-registry-credentials",
		})
	}

	return objects, nil
}

func shootObjects() []client.Object {
	return []client.Object{
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system:group-rolebinding-controller",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "system:group-rolebinding-controller",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
		},
	}
}
