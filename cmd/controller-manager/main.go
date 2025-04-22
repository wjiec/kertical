/*
Copyright 2025 Jayson Wang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"

	"github.com/pkg/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/wjiec/kertical/internal/bootstrap"
	"github.com/wjiec/kertical/internal/controller"
	"github.com/wjiec/kertical/internal/fieldindex"
)

func main() {
	bootstrap.New().RunForever(ctrl.SetupSignalHandler(),
		bootstrap.WithSetupLogging(zap.Options{Development: true}),
		bootstrap.WithHealthProbe(":8081"),
		bootstrap.WithMetricsServer(false, "0", false),
		bootstrap.WithLeaderElection(false, "kertical-manager"),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			log.FromContext(ctx).Info("registering field-index for resources")
			if err := fieldindex.RegisterFieldIndexes(ctx, mgr.GetFieldIndexer()); err != nil {
				return errors.Wrap(err, "unable to register field index")
			}

			return nil
		}),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			log.FromContext(ctx).Info("create and register controllers to the manager")
			if err := controller.SetupWithManager(mgr); err != nil {
				return errors.Wrap(err, "unable to create or register controllers")
			}

			return nil
		}),
	)
}
