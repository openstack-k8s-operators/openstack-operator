/*
Copyright 2026.

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

package openstack

import (
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
)

func TestGetContainerImagesDoesNotMutateCustomImageSpec(t *testing.T) {
	g := NewWithT(t)
	cinderImage := "custom.registry/cinder-volume:custom-tag"
	manilaImage := "custom.registry/manila-share:custom-tag"
	cinderDefault := "default.registry/cinder-volume:tag"
	manilaDefault := "default.registry/manila-share:tag"

	instance := corev1.OpenStackVersion{
		Spec: corev1.OpenStackVersionSpec{
			CustomContainerImages: corev1.CustomContainerImages{
				CinderVolumeImages: map[string]*string{"backend1": &cinderImage},
				ManilaShareImages:  map[string]*string{"share-backend1": &manilaImage},
			},
		},
	}

	images := GetContainerImages(&corev1.ContainerDefaults{
		CinderVolumeImage: &cinderDefault,
		ManilaShareImage:  &manilaDefault,
	}, instance)

	g.Expect(instance.Spec.CustomContainerImages.CinderVolumeImages).To(HaveLen(1))
	g.Expect(instance.Spec.CustomContainerImages.CinderVolumeImages).NotTo(HaveKey("default"))
	g.Expect(instance.Spec.CustomContainerImages.ManilaShareImages).To(HaveLen(1))
	g.Expect(instance.Spec.CustomContainerImages.ManilaShareImages).NotTo(HaveKey("default"))
	g.Expect(images.CinderVolumeImages).To(HaveKeyWithValue("default", &cinderDefault))
	g.Expect(images.ManilaShareImages).To(HaveKeyWithValue("default", &manilaDefault))
}
