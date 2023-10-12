package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the fi-ts authn provider.
type ControllerConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// Auth is the configuration for fi-ts specific user authentication in the cluster.
	Auth Auth `json:"auth"`

	// HealthCheckConfig is the config for the health check controller
	// +optional
	HealthCheckConfig *healthcheckconfigv1alpha1.HealthCheckConfig `json:"healthCheckConfig,omitempty"`

	// ImagePullSecret provides an opportunity to inject an image pull secret into the resource deployments
	ImagePullSecret *ImagePullSecret `json:"imagePullSecret,omitempty"`
}

// Auth contains the configuration for fi-ts specific user authentication in the cluster.
type Auth struct {
	// ProviderTenant is the name of the provider tenant who has special privileges.
	ProviderTenant string `json:"providerTenant"`

	MetalURL      string `json:"metalURL"`
	MetalHMAC     string `json:"metalHMAC"`
	MetalAuthType string `json:"metalAuthType"`
}

// ImagePullSecret provides an opportunity to inject an image pull secret into the resource deployments
type ImagePullSecret struct {
	// DockerConfigJSON contains the already base64 encoded JSON content for the image pull secret
	DockerConfigJSON string `json:"encodedDockerConfigJSON"`
}
