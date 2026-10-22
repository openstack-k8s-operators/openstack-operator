/*
Copyright 2024.

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

package deployment

import (
	"testing"

	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
)

func Test_selectOSImage(t *testing.T) {
	uefi := "quay.io/podified-antelope-centos9/edpm-hardened-uefi:current-podified"
	bootc := "quay.io/openstack-k8s-operators/edpm-bootc:latest-qcow2"
	images := openstackv1.ContainerImages{
		ContainerTemplate: openstackv1.ContainerTemplate{
			OsContainerImage:      &uefi,
			OsBootcContainerImage: &bootc,
		},
	}

	cases := []struct {
		name   string
		imgTyp dataplanev1.OSImageType
		want   *string
	}{
		{"explicit uefi", dataplanev1.OSImageTypeUEFI, &uefi},
		{"empty defaults to uefi", "", &uefi},
		{"bootc", dataplanev1.OSImageTypeBootc, &bootc},
		{"unknown defaults to uefi", "garbage", &uefi},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectOSImage(tc.imgTyp, images); got != tc.want {
				t.Fatalf("selectOSImage(%q) = %q, want %q", tc.imgTyp, deref(got), deref(tc.want))
			}
		})
	}
}

func Test_selectOSImage_nilBootc(t *testing.T) {
	uefi := "quay.io/podified-antelope-centos9/edpm-hardened-uefi:current-podified"
	// bootc image unset (nil); requesting bootc must return nil, not panic.
	images := openstackv1.ContainerImages{
		ContainerTemplate: openstackv1.ContainerTemplate{
			OsContainerImage: &uefi,
		},
	}
	if got := selectOSImage(dataplanev1.OSImageTypeBootc, images); got != nil {
		t.Fatalf("selectOSImage(bootc) with nil bootc image = %q, want nil", deref(got))
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
