//go:generate sh -c "bash $GARDENER_HACK_DIR/generate-controller-registration.sh fits-authn . $(cat ../../VERSION) ../../example/controller-registration.yaml Extension:fits-authn"

// Package chart enables go:generate support for generating the correct controller registration.
package chart
