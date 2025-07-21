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
	"flag"
	"os"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

// Bootstrap is a dynamically configurable tool for initializing manager setups.
type Bootstrap struct {
	scheme   *runtime.Scheme
	setupLog logr.Logger

	ctrlOptions  ctrl.Options
	beforeCreate []func(context.Context) error
	beforeStart  []func(context.Context, ctrl.Manager) error
	beforeStop   []func(context.Context, ctrl.Manager) error
}

// New initializes and returns a new Bootstrap instance with a runtime scheme and logger.
func New() *Bootstrap {
	scheme := runtime.NewScheme()
	metav1.AddToGroupVersion(scheme, metav1.SchemeGroupVersion)
	utilruntime.Must(kubescheme.AddToScheme(scheme))
	utilruntime.Must(networkingv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	return &Bootstrap{
		scheme:   scheme,
		setupLog: ctrl.Log.WithName("setup"),
	}
}

// addBeforeCreate adds a hook function to be executed before manager creation.
func (b *Bootstrap) addBeforeCreate(hook func(context.Context) error) {
	b.beforeCreate = append(b.beforeCreate, hook)
}

// addBeforeStart adds a hook function to be executed before the manager starts.
func (b *Bootstrap) addBeforeStart(hook func(context.Context, ctrl.Manager) error) {
	b.beforeStart = append(b.beforeStart, hook)
}

// addBeforeStart adds a hook function to be executed before the manager stops.
func (b *Bootstrap) addBeforeStop(hook func(context.Context, ctrl.Manager) error) {
	b.beforeStop = append(b.beforeStop, hook)
}

// RunForever sets up the manager and runs it indefinitely, processing options and hooks beforehand.
func (b *Bootstrap) RunForever(ctx context.Context, options ...Option) {
	ctx = log.IntoContext(ctx, b.setupLog)
	for _, configure := range options {
		configure(b)
	}

	// Parse command-line flags
	flag.Parse()

	// Execute all hooks added to beforeCreate, checking for errors in the process.
	for _, beforeCreate := range b.beforeCreate {
		if err := beforeCreate(ctx); err != nil {
			b.setupLog.Error(err, "failed to run beforeCreate hook")
			os.Exit(1)
		}
	}

	// Initialize a new manager instance with the desired configuration options.
	b.ctrlOptions.Scheme = b.scheme
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), b.ctrlOptions)
	if err != nil {
		b.setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Execute all hooks added to beforeStart, passing the manager to each.
	for _, beforeStart := range b.beforeStart {
		if err = beforeStart(ctx, mgr); err != nil {
			b.setupLog.Error(err, "failed to run beforeStart hook")
			os.Exit(1)
		}
	}

	// Execute all hooks added to beforeStop, passing the manager to each.
	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		<-ctx.Done()

		for _, beforeStop := range b.beforeStop {
			if err := beforeStop(ctx, mgr); err != nil {
				b.setupLog.Error(err, "failed to run beforeStop hook")
				return err
			}
		}
		return nil
	}))
	if err != nil {
		b.setupLog.Error(err, "unable to add beforeStop hooks to the manager")
		os.Exit(1)
	}

	b.setupLog.Info("starting manager")
	if err = mgr.Start(ctx); err != nil {
		b.setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
