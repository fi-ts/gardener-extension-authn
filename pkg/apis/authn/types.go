package authn

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthnConfig configuration resource
type AuthnConfig struct {
	metav1.TypeMeta

	Issuer   string
	ClientID string
}
