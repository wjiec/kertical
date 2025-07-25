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

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/wjiec/kertical/internal/controller/networking"
)

// ctrlSetups is a slice of controller setup functions
var ctrlSetups []func(mgr manager.Manager) error

func init() {
	ctrlSetups = append(ctrlSetups, networking.SetupExternalProxy)
}

// SetupWithManager initializes all registered controller setup function with the provided manager
//
// Returns an error if any controller fails to initialize
func SetupWithManager(mgr ctrl.Manager) error {
	for _, setupController := range ctrlSetups {
		if err := setupController(mgr); err != nil {
			return err
		}
	}

	return nil
}
