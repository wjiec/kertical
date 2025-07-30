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

package kertical

import "os"

// ControllerName returns the name of the current controller.
func ControllerName() string {
	return "kertical"
}

// GetNamespace retrieves the namespace in which the pod is running.
func GetNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}

	return "kertical-system"
}

// NodeName retrieves the name of the current node from the NODE_NAME environment variable.
//
// This environment variable is injected by Kubernetes when running in a Pod with the downward API.
func NodeName() string {
	if nn := os.Getenv("NODE_NAME"); nn != "" {
		return nn
	}
	return "unknown-node"
}

// NodeIP retrieves the IP address of the current node from the NODE_IP environment variable.
//
// If the environment variable is not set or empty, it defaults to "127.0.0.1".
// This environment variable is typically injected by Kubernetes when running in a Pod
// with the downward API.
func NodeIP() string {
	if ni := os.Getenv("NODE_IP"); len(ni) != 0 {
		return ni
	}
	return "127.0.0.1"
}
