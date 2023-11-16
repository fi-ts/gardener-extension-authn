package controller

import (
	"context"
	"encoding/base64"
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
	"github.com/gardener/gardener/pkg/extensions"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/go-logr/logr"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(config config.ControllerConfiguration) extension.Actuator {
	return &actuator{
		config: config,
	}
}

type actuator struct {
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
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.GetNamespace()

	cluster, err := controller.GetCluster(ctx, a.client, namespace)
	if err != nil {
		return err
	}

	authnConfig := &v1alpha1.AuthnConfig{}
	if ex.Spec.ProviderConfig != nil {
		if _, _, err := a.decoder.Decode(ex.Spec.ProviderConfig.Raw, nil, authnConfig); err != nil {
			return fmt.Errorf("failed to decode provider config: %w", err)
		}
	}

	if err := a.createResources(ctx, log, authnConfig, cluster, namespace); err != nil {
		return err
	}

	return nil
}

// Delete the Extension resource.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.deleteResources(ctx, log, ex.GetNamespace())
}

// Restore the Extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, log, ex)
}

// Migrate the Extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return nil
}

func (a *actuator) createResources(ctx context.Context, log logr.Logger, authConfig *v1alpha1.AuthnConfig, cluster *controller.Cluster, namespace string) error {
	shootAccessSecret := gutil.NewShootAccessSecret(gutil.SecretNamePrefixShootAccess+"group-rolebinding-controller", namespace)
	if err := shootAccessSecret.Reconcile(ctx, a.client); err != nil {
		return err
	}

	shootObjects := shootObjects()

	seedObjects, err := seedObjects(&a.config, authConfig, cluster, namespace, shootAccessSecret.Secret.Name)
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

	if err := managedresources.CreateForShoot(ctx, a.client, namespace, v1alpha1.ShootAuthResourceName, "fits-authn", false, shootResources); err != nil {
		return err
	}

	log.Info("managed resource created successfully", "name", v1alpha1.ShootAuthResourceName)

	if err := managedresources.CreateForSeed(ctx, a.client, namespace, v1alpha1.SeedAuthResourceName, false, seedResources); err != nil {
		return err
	}

	log.Info("managed resource created successfully", "name", v1alpha1.SeedAuthResourceName)

	return nil
}

func (a *actuator) deleteResources(ctx context.Context, log logr.Logger, namespace string) error {
	log.Info("deleting managed resource for registry cache")

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

func seedObjects(cc *config.ControllerConfiguration, authConfig *v1alpha1.AuthnConfig, cluster *controller.Cluster, namespace, shootAccessSecretName string) ([]client.Object, error) {
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
						"networking.gardener.cloud/from-prometheus":      "allowed",
						"networking.gardener.cloud/from-shoot-apiserver": "allowed",
						"networking.gardener.cloud/to-dns":               "allowed",
						"networking.gardener.cloud/to-public-networks":   "allowed",
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
							Image:           authnImage.String(),
							ImagePullPolicy: corev1.PullIfNotPresent,
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
									Value: cluster.Shoot.Name,
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
						},
					},
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
							Image:           grcImage.String(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"/group-rolebinding-controller"},
							Args: []string{
								"--excludeNamespaces=kube-system,kube-public,kube-node-lease,default",
								"--expectedGroupsList=admin,edit,view",
								fmt.Sprintf("--clustername=%s", cluster.Shoot.Name),
								fmt.Sprintf("--kubeconfig=%s", gutil.PathGenericKubeconfig),
							},
						},
					},
				},
			},
		},
	}

	if err := gutil.InjectGenericKubeconfig(grcDeployment, extensions.GenericTokenKubeconfigSecretNameFromCluster(cluster), shootAccessSecretName); err != nil {
		return nil, err
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
	}

	if cc.ImagePullSecret != nil && cc.ImagePullSecret.DockerConfigJSON != "" {
		content, err := base64.StdEncoding.DecodeString(cc.ImagePullSecret.DockerConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("unable to decode image pull secret: %w", err)
		}

		objects = append(objects, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-jwt-authn-webhook-registry-credentials",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "kube-jwt-authn-webhook-registry-credentials",
				},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				".dockerconfigjson": content,
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
			Data: map[string][]byte{
				".dockerconfigjson": content,
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
					Name: "system:serviceaccount:kube-system:group-rolebinding-controller",
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
