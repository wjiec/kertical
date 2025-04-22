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

package bootstrap

import (
	"context"
	"crypto/tls"
	"flag"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	kerticalwebhook "github.com/wjiec/kertical/internal/webhook"
)

// Option represents a function that can be used to configure a Bootstrap instance.
type Option func(*Bootstrap)

// WithBeforeCreate adds a hook to be executed before manager creation in the Bootstrap process.
func WithBeforeCreate(hook func(context.Context) error) Option {
	return func(bootstrap *Bootstrap) {
		bootstrap.addBeforeCreate(hook)
	}
}

// WithBeforeStart adds a hook to be executed before the manager starts.
func WithBeforeStart(hook func(context.Context, ctrl.Manager) error) Option {
	return func(bootstrap *Bootstrap) {
		bootstrap.addBeforeStart(hook)
	}
}

// WithSetupLogging configures logging setup using the provided zap options.
func WithSetupLogging(options zap.Options) Option {
	return func(bootstrap *Bootstrap) {
		options.BindFlags(flag.CommandLine)

		bootstrap.addBeforeCreate(func(ctx context.Context) error {
			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options)))
			return nil
		})
	}
}

// WithHealthProbe configures the health probe server with the specified address.
func WithHealthProbe(addr string) Option {
	return func(bootstrap *Bootstrap) {
		realAddr := flag.String("health-probe-bind-address", addr, "The address the probe endpoint binds to")

		bootstrap.addBeforeCreate(func(ctx context.Context) error {
			bootstrap.ctrlOptions.HealthProbeBindAddress = *realAddr
			return nil
		})

		bootstrap.addBeforeStart(func(ctx context.Context, mgr ctrl.Manager) error {
			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				return errors.Wrap(err, "unable to set up health check")
			}

			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				return errors.Wrap(err, "unable to set up ready check")
			}

			return nil
		})
	}
}

// WithMetricsServer sets up the metrics server based on the provided configurations.
func WithMetricsServer(secure bool, addr string, http2 bool) Option {
	return func(bootstrap *Bootstrap) {
		realAddr := flag.String("metrics-bind-address", addr, "The address the metric endpoint binds to. Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
		realSecure := flag.Bool("metrics-secure", secure, "If set, the metrics endpoint is served securely via HTTPS")
		realHTTP2 := flag.Bool("metrics-http2", http2, "If set, HTTP/2 will be enabled for the metrics servers")

		bootstrap.addBeforeCreate(func(ctx context.Context) error {
			metricsServerOptions := metricsserver.Options{
				BindAddress:   *realAddr,
				SecureServing: *realSecure,
				TLSOpts:       createTLSOpts(*realHTTP2),
			}

			if *realSecure {
				// FilterProvider is used to protect the metrics endpoint with authn/authz.
				// These configurations ensure that only authorized users and service accounts
				// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
				// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
				metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization

				// TODO(user): If CertDir, CertName, and KeyName are not specified, controller-runtime will automatically
				// generate self-signed certificates for the metrics server. While convenient for development and testing,
				// this setup is not recommended for production.

				// TODO(user): If cert-manager is enabled in config/default/kustomization.yaml,
				// you can uncomment the following lines to use the certificate managed by cert-manager.
				// metricsServerOptions.CertDir = "/tmp/k8s-metrics-server/metrics-certs"
				// metricsServerOptions.CertName = "tls.crt"
				// metricsServerOptions.KeyName = "tls.key"
				metricsServerOptions.CertDir = "/tmp/kertical-metrics-certs"
			}

			bootstrap.ctrlOptions.Metrics = metricsServerOptions
			return nil
		})
	}
}

// WithWebhookServer configures the webhook server for the controller manager.
func WithWebhookServer(http2 bool) Option {
	return func(bootstrap *Bootstrap) {
		realHTTP2 := flag.Bool("webhook-http2", http2, "If set, HTTP/2 will be enabled for the webhook servers")

		bootstrap.addBeforeCreate(func(ctx context.Context) error {
			bootstrap.ctrlOptions.WebhookServer = webhook.NewServer(webhook.Options{
				CertDir: kerticalwebhook.GetCertDir(),
				TLSOpts: createTLSOpts(*realHTTP2),
			})

			return nil
		})
	}
}

// WithLeaderElection configures leader election for the controller manager to ensure only one
// active instance at a time. Leader election helps prevent multiple instances of the manager
// from performing the same operations concurrently.
func WithLeaderElection(elect bool, electionId string) Option {
	return func(bootstrap *Bootstrap) {
		realElect := flag.Bool("leader-elect", elect, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager")
		realElectionId := flag.String("leader-election-id", electionId, "Determines the name of the resource that leader election will use for holding the leader lock")

		bootstrap.addBeforeCreate(func(ctx context.Context) error {
			bootstrap.ctrlOptions.LeaderElection = *realElect
			bootstrap.ctrlOptions.LeaderElectionID = *realElectionId

			return nil
		})
	}
}

// createTLSOpts creates a list of TLS options based on the provided configurations.
func createTLSOpts(enableHTTP2 bool) []func(*tls.Config) {
	var tlsOpts []func(*tls.Config)
	if !enableHTTP2 {
		// if the enable-http2 flag is false (the default), http/2 should be disabled
		// due to its vulnerabilities. More specifically, disabling http/2 will
		// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
		// Rapid Reset CVEs. For more information see:
		// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
		// - https://github.com/advisories/GHSA-4374-p667-p6c8
		tlsOpts = append(tlsOpts, func(c *tls.Config) {
			c.NextProtos = []string{"http/1.1"}
		})
	}

	return tlsOpts
}
