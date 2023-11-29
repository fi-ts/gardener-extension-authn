#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

rm -f $GOPATH/bin/*-gen

PROJECT_ROOT=$(dirname $0)/..

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/fi-ts/gardener-extension-authn/pkg/client \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "authn:v1alpha1" \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/fi-ts/gardener-extension-authn/pkg/client \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "authn:v1alpha1" \
  --extra-peer-dirs=github.com/fi-ts/gardener-extension-authn/pkg/apis/authn,github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  deepcopy,defaulter \
  github.com/fi-ts/gardener-extension-authn/pkg/client/componentconfig \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "config:v1alpha1" \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"

bash "${PROJECT_ROOT}"/vendor/k8s.io/code-generator/generate-internal-groups.sh \
  conversion \
  github.com/fi-ts/gardener-extension-authn/pkg/client/componentconfig \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  github.com/fi-ts/gardener-extension-authn/pkg/apis \
  "config:v1alpha1" \
  --extra-peer-dirs=github.com/fi-ts/gardener-extension-authn/pkg/apis/config,github.com/fi-ts/gardener-extension-authn/pkg/apis/config/v1alpha1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
  --go-header-file "${PROJECT_ROOT}/hack/boilerplate.txt"
