#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# setup virtual GOPATH
source "$GARDENER_HACK_DIR"/vgopath-setup.sh

CODE_GEN_DIR=$(go list -m -f '{{.Dir}}' k8s.io/code-generator)

# We need to explicitly pass GO111MODULE=off to k8s.io/code-generator as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
export GO111MODULE=off

rm -f $GOPATH/bin/*-gen

PROJECT_ROOT=$(dirname $0)/..

git config --global --add safe.directory /go/src/github.com/fi-ts/gardener-extension-authn

bash "${CODE_GEN_DIR}/generate-internal-groups.sh" \
  deepcopy,defaulter \
  github.com/fi-ts/gardener-extension-authn/pkg/client \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "authn:v1alpha1" \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${CODE_GEN_DIR}/generate-internal-groups.sh" \
  conversion \
  github.com/fi-ts/gardener-extension-authn/pkg/client \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "authn:v1alpha1" \
  --extra-peer-dirs=github.com/fi-ts/gardener-extension-authn/pkg/apis/authn,github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${CODE_GEN_DIR}/generate-internal-groups.sh" \
  deepcopy,defaulter \
  github.com/fi-ts/gardener-extension-authn/pkg/client/componentconfig \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "config:v1alpha1" \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${CODE_GEN_DIR}/generate-internal-groups.sh" \
  conversion \
  github.com/fi-ts/gardener-extension-authn/pkg/client/componentconfig \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/fi-ts/gardener-extension-authn/pkg/apis/config,github.com/fi-ts/gardener-extension-authn/pkg/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"
