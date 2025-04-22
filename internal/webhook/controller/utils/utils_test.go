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

package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/wjiec/kertical/internal/webhook/controller/utils"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Controller Suite")
}

var _ = Describe("DefaultContainer", func() {
	var (
		containers []corev1.Container
	)

	BeforeEach(func() {
		containers = []corev1.Container{
			{Name: "container1"},
			{Name: "container2"},
		}
	})

	Context("when the object has a default container annotation", func() {
		It("should return the default container annotation", func() {
			object := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utils.DefaultContainerAnnotation: "container2",
					},
				},
			}

			defaultContainer := utils.DefaultContainer(object, containers)
			Expect(defaultContainer).To(Equal(containers[1]))
		})
	})

	Context("when the annotation does not match any container", func() {
		It("should return the first container", func() {
			object := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utils.DefaultContainerAnnotation: "container3",
					},
				},
			}

			defaultContainer := utils.DefaultContainer(object, containers)
			Expect(defaultContainer).To(Equal(containers[0]))
		})
	})

	Context("when the object has no default container annotation", func() {
		It("should return the first container", func() {
			object := &corev1.Pod{}

			defaultContainer := utils.DefaultContainer(object, containers)
			Expect(defaultContainer).To(Equal(containers[0]))
		})
	})
})

var _ = Describe("MountedVolume", func() {
	var container *corev1.Container

	BeforeEach(func() {
		container = &corev1.Container{
			VolumeMounts: []corev1.VolumeMount{
				{Name: "foo", MountPath: "/foo"},
				{Name: "bar", MountPath: "/bar"},
			},
		}
	})

	Context("when volume is mounted at the specified path", func() {
		It("should return the name of the volume and true", func() {
			volumeName, found := utils.MountedVolume(container, "/foo")
			Expect(found).To(BeTrue())
			Expect(volumeName).To(Equal("foo"))
		})
	})

	Context("when no volume is mounted at the specified path", func() {
		It("should return an empty name and false", func() {
			volumeName, found := utils.MountedVolume(container, "/baz")
			Expect(found).To(BeFalse())
			Expect(volumeName).To(BeEmpty())
		})
	})
})
