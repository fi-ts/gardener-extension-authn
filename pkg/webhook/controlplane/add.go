package controlplane

import (
	"github.com/fi-ts/gardener-extension-authn/pkg/apis/config"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
	logger            = log.Log.WithName("fits-authn-controlplane-webhook")
)

// AddOptions are options to apply when adding the metal infrastructure controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	ControllerConfig config.ControllerConfiguration
}

func AddToManagerWithOptions(mgr manager.Manager, opts AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return controlplane.New(mgr, controlplane.Args{
		Kind:     controlplane.KindShoot,
		Provider: "metal",
		Types: []extensionswebhook.Type{
			{Obj: &appsv1.Deployment{}},
		},
		Mutator: genericmutator.NewMutator(NewEnsurer(logger, opts.ControllerConfig), nil, nil, nil, logger),
	})
}

// AddToManager creates a webhook and adds it to the manager.
func AddToManager(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return AddToManagerWithOptions(mgr, DefaultAddOptions)
}
