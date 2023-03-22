//go:generate sh -c "../../vendor/github.com/gardener/gardener/hack/generate-controller-registration.sh extension-fits-authn . $(cat ../../VERSION) ../../example/controller-registration.yaml Extension:fits-authn"

// Package chart enables go:generate support for generating the correct controller registration.
package chart
