package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthcheckconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControllerConfiguration defines the configuration for the fi-ts authn provider.
type ControllerConfiguration struct {
	metav1.TypeMeta

	// Auth is the configuration for fi-ts specific user authentication in the cluster.
	Auth Auth

	// HealthCheckConfig is the config for the health check controller
	HealthCheckConfig *healthcheckconfig.HealthCheckConfig

	// ImagePullSecret provides an opportunity to inject an image pull secret into the resource deployments
	ImagePullSecret *ImagePullSecret
}

// Auth contains the configuration for fi-ts specific user authentication in the cluster.
type Auth struct {
	// ProviderTenant is the name of the provider tenant who has special privileges.
	ProviderTenant string

	MetalURL      string
	MetalHMAC     string
	MetalAuthType string
}

// ImagePullSecret provides an opportunity to inject an image pull secret into the resource deployments
type ImagePullSecret struct {
	// DockerConfigJSON contains the already base64 encoded JSON content for the image pull secret
	DockerConfigJSON string
}
