package app

import (
	"os"

	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	authncmd "github.com/fi-ts/gardener-extension-authn/pkg/cmd"
)

// ExtensionName is the name of the extension.
const ExtensionName = "extension-fits-authn"

// Options holds configuration passed to the registry service controller.
type Options struct {
	generalOptions     *controllercmd.GeneralOptions
	authnOptions       *authncmd.AuthOptions
	restOptions        *controllercmd.RESTOptions
	managerOptions     *controllercmd.ManagerOptions
	controllerOptions  *controllercmd.ControllerOptions
	healthOptions      *controllercmd.ControllerOptions
	controllerSwitches *controllercmd.SwitchOptions
	webhookOptions     *webhookcmd.AddToManagerOptions
	reconcileOptions   *controllercmd.ReconcilerOptions
	optionAggregator   controllercmd.OptionAggregator
}

// NewOptions creates a new Options instance.
func NewOptions() *Options {
	// options for the webhook server
	webhookServerOptions := &webhookcmd.ServerOptions{
		Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
	}

	webhookSwitches := authncmd.WebhookSwitchOptions()
	webhookOptions := webhookcmd.NewAddToManagerOptions(
		"fits-authn",
		webhookServerOptions,
		webhookSwitches,
	)

	options := &Options{
		generalOptions: &controllercmd.GeneralOptions{},
		authnOptions:   &authncmd.AuthOptions{},
		restOptions:    &controllercmd.RESTOptions{},
		managerOptions: &controllercmd.ManagerOptions{
			// These are default values.
			LeaderElection:             true,
			LeaderElectionID:           controllercmd.LeaderElectionNameID(ExtensionName),
			LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
			LeaderElectionNamespace:    os.Getenv("LEADER_ELECTION_NAMESPACE"),
		},
		controllerOptions: &controllercmd.ControllerOptions{
			// This is a default value.
			MaxConcurrentReconciles: 5,
		},
		healthOptions: &controllercmd.ControllerOptions{
			// This is a default value.
			MaxConcurrentReconciles: 5,
		},
		controllerSwitches: authncmd.ControllerSwitchOptions(),
		reconcileOptions:   &controllercmd.ReconcilerOptions{},
		webhookOptions:     webhookOptions,
	}

	options.optionAggregator = controllercmd.NewOptionAggregator(
		options.generalOptions,
		options.restOptions,
		options.managerOptions,
		options.controllerOptions,
		options.authnOptions,
		controllercmd.PrefixOption("healthcheck-", options.healthOptions),
		options.controllerSwitches,
		options.reconcileOptions,
		options.webhookOptions,
	)

	return options
}
