package app

import (
	"context"
	"fmt"

	"github.com/fi-ts/gardener-extension-authn/pkg/apis/authn/install"
	"github.com/fi-ts/gardener-extension-authn/pkg/controller"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	heartbeatcontroller "github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewControllerManagerCommand creates a new command that is used to start the controller.
func NewControllerManagerCommand() *cobra.Command {
	options := NewOptions()

	cmd := &cobra.Command{
		Use:           "gardener-extension-authn",
		Short:         "provides cluster authentication and authorization in the shoot clusters.",
		SilenceErrors: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.optionAggregator.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			if err := options.heartbeatOptions.Validate(); err != nil {
				return err
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

	if mgrOpts.Client.Cache == nil {
		mgrOpts.Client.Cache = &client.CacheOptions{}
	}

	mgrOpts.Client.Cache.DisableFor = []client.Object{
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
	o.controllerOptions.Completed().Apply(&controller.DefaultAddOptions.ControllerOptions)
	o.reconcileOptions.Completed().Apply(&controller.DefaultAddOptions.IgnoreOperationAnnotation, &controller.DefaultAddOptions.ExtensionClass)
	o.heartbeatOptions.Completed().Apply(&heartbeatcontroller.DefaultAddOptions)

	if err := o.controllerSwitches.Completed().AddToManager(ctx, mgr); err != nil {
		return fmt.Errorf("could not add controllers to manager: %w", err)
	}

	if _, err := o.webhookOptions.Completed().AddToManager(ctx, mgr, nil); err != nil {
		return fmt.Errorf("could not add the mutating webhook to manager: %w", err)
	}

	if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
		return fmt.Errorf("could not add ready check for informers: %w", err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("could not add health check to manager: %w", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("error running manager: %w", err)
	}

	return nil
}
