package app

import (
	"context"
	"fmt"

	"github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/install"
	"github.com/fi-ts/gardener-extension-authn/pkg/controller"

	webhook "github.com/fi-ts/gardener-extension-authn/pkg/webhook/controlplane"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewControllerManagerCommand creates a new command that is used to start the controller.
func NewControllerManagerCommand() *cobra.Command {
	options := NewOptions()

	cmd := &cobra.Command{
		Use:           "gardener-extension-authn",
		Short:         "rovides cluster authentication and authorization in the shoot clusters.",
		SilenceErrors: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.optionAggregator.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			cmd.SilenceUsage = true
			return options.run(cmd.Context())
		},
	}

	options.optionAggregator.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) run(ctx context.Context) error {
	// TODO: Make these flags configurable via command line parameters or component config file.
	util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfig.ClientConnectionConfiguration{
		QPS:   100.0,
		Burst: 130,
	}, o.restOptions.Completed().Config)

	mgrOpts := o.managerOptions.Completed().Options()

	mgrOpts.ClientDisableCacheFor = []client.Object{
		&corev1.Secret{},    // applied for ManagedResources
		&corev1.ConfigMap{}, // applied for monitoring config
	}

	mgr, err := manager.New(o.restOptions.Completed().Config, mgrOpts)
	if err != nil {
		return fmt.Errorf("could not instantiate controller-manager: %w", err)
	}

	if err := extensionscontroller.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("could not update manager scheme: %w", err)
	}

	if err := install.AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("could not update manager scheme: %w", err)
	}

	ctrlConfig := o.authnOptions.Completed()
	ctrlConfig.Apply(&controller.DefaultAddOptions.Config)
	ctrlConfig.Apply(&webhook.DefaultAddOptions.ControllerConfig)
	o.controllerOptions.Completed().Apply(&controller.DefaultAddOptions.ControllerOptions)
	o.reconcileOptions.Completed().Apply(&controller.DefaultAddOptions.IgnoreOperationAnnotation)

	if err := o.controllerSwitches.Completed().AddToManager(mgr); err != nil {
		return fmt.Errorf("could not add controllers to manager: %w", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("error running manager: %w", err)
	}

	return nil
}
