package main

import (
	"github.com/fi-ts/gardener-extension-authn/cmd/gardener-extension-authn/app"

	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	log "github.com/gardener/gardener/pkg/logger"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
)

func main() {
	runtimelog.SetLogger(log.ZapLogger(false))
	cmd := app.NewControllerManagerCommand()

	if err := cmd.Execute(); err != nil {
		controllercmd.LogErrAndExit(err, "error executing the main controller command")
	}
}
