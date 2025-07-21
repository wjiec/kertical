package main

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/wjiec/kertical/internal/bootstrap"
	"github.com/wjiec/kertical/internal/controller/networking"
)

func main() {
	bootstrap.New().RunForever(ctrl.SetupSignalHandler(),
		bootstrap.WithSetupLogging(zap.Options{Development: true}),
		bootstrap.WithHealthProbe(":8081"),
		bootstrap.WithMetricsServer(false, "0", false),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			log.FromContext(ctx).Info("create and register controllers to the manager")
			if err := networking.SetupPortForwarding(mgr); err != nil {
				return errors.Wrap(err, "unable to create or register controllers")
			}
			return nil
		}),
	)
}
