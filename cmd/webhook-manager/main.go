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
	"os"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/wjiec/kertical/internal/bootstrap"
	kerticalwebhook "github.com/wjiec/kertical/internal/webhook"
	webhookcontroller "github.com/wjiec/kertical/internal/webhook/controller"
)

func main() {
	bootstrap.New().RunForever(ctrl.SetupSignalHandler(),
		bootstrap.WithSetupLogging(zap.Options{Development: true}),
		bootstrap.WithHealthProbe(":8081"),
		bootstrap.WithWebhookServer(false),
		bootstrap.WithMetricsServer(false, "0", false),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			if _, found := os.LookupEnv("DISABLED_CONTROLLER"); !found {
				return webhookcontroller.SetupWithManager(mgr)
			}
			return nil
		}),
		bootstrap.WithBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			if err := kerticalwebhook.SetupWebhookWithManager(mgr); err != nil {
				return errors.Wrap(err, "unable to setup webhooks")
			}

			return nil
		}),
	)
}
