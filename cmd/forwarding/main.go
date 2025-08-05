package main

import (
	"context"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/wjiec/kertical/internal/bootstrap"
	"github.com/wjiec/kertical/internal/controller/networking"
	"github.com/wjiec/kertical/internal/controller/networking/portforwarding"
)

func main() {
	var stop func() error
	bootstrap.New().RunForever(ctrl.SetupSignalHandler(),
		bootstrap.WithSetupLogging(zap.Options{Development: true}),
		bootstrap.WithHealthProbe(":8081"),
		bootstrap.WithMetricsServer(false, "0", false),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			log.FromContext(ctx).Info("registering field-index for resources")
			if err := portforwarding.RegisterFieldIndexes(ctx, mgr.GetFieldIndexer()); err != nil {
				return errors.Wrap(err, "unable to register field index")
			}

			return nil
		}),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) (err error) {
			log.FromContext(ctx).Info("create and register controllers to the manager")
			if stop, err = networking.SetupPortForwarding(mgr); err != nil {
				return errors.Wrap(err, "unable to create or register controllers")
			}
			return nil
		}),
		bootstrap.WithBeforeStop(func(ctx context.Context, mgr ctrl.Manager) error {
			log.FromContext(ctx).Info("stop controller manager")
			return stop()
		}),
	)
}
