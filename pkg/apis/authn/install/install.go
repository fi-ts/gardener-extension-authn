package install

import (
	"github.com/fi-ts/gardener-extension-authn/pkg/apis/authn"
	"github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		v1alpha1.AddToScheme,
		authn.AddToScheme,
		setVersionPriority,
	)

	// AddToScheme adds all APIs to the scheme.
	AddToScheme = schemeBuilder.AddToScheme
)

func setVersionPriority(scheme *runtime.Scheme) error {
	return scheme.SetVersionPriority(v1alpha1.SchemeGroupVersion)
}

// Install installs all APIs in the scheme.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(AddToScheme(scheme))
}
