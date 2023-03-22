package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AuthResourceName = "extension-fits-auth"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthnConfig configuration resource
type AuthnConfig struct {
	metav1.TypeMeta `json:",inline"`

	Issuer string `json:"issuer,omitempty"`

	ClientID string `json:"clientID,omitempty"`
}
