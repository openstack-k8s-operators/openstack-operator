package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DummyObject is a mock for testing with objects that are not OpenStackVersion
type DummyObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (d *DummyObject) DeepCopyObject() runtime.Object {
	copy := *d
	return &copy
}

var _ = Describe("OpenStackVersion Webhook", func() {

	Context("ValidateUpdate with annotation support", func() {

		var (
			oldVersion *OpenStackVersion
			newVersion *OpenStackVersion
		)

		BeforeEach(func() {
			// Set up test defaults to make tests work
			SetupOpenStackVersionDefaults(OpenStackVersionDefaults{
				AvailableVersion: "1.1.0",
			})

			// Setup a base version with deployed status and custom images
			cinderImage := "registry.example.com/cinder-volume:backend1-v1.0.0"
			keystoneImage := "registry.example.com/keystone:v1.0.0"

			oldVersion = &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{
					TargetVersion: "1.0.0",
					CustomContainerImages: CustomContainerImages{
						ContainerTemplate: ContainerTemplate{
							KeystoneAPIImage: &keystoneImage,
						},
						CinderVolumeImages: map[string]*string{
							"backend1": &cinderImage,
						},
					},
				},
				Status: OpenStackVersionStatus{
					DeployedVersion: &[]string{"1.0.0"}[0],
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": &ContainerDefaults{},
						"1.1.0": &ContainerDefaults{},
					},
					TrackedCustomImages: map[string]CustomContainerImages{
						"1.0.0": {
							ContainerTemplate: ContainerTemplate{
								KeystoneAPIImage: &keystoneImage,
							},
							CinderVolumeImages: map[string]*string{
								"backend1": &cinderImage,
							},
						},
					},
				},
			}

			// Setup new version with changed target version but same custom images
			newVersion = &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{
					TargetVersion: "1.1.0",
					CustomContainerImages: CustomContainerImages{
						ContainerTemplate: ContainerTemplate{
							KeystoneAPIImage: &keystoneImage,
						},
						CinderVolumeImages: map[string]*string{
							"backend1": &cinderImage,
						},
					},
				},
				Status: OpenStackVersionStatus{
					DeployedVersion: &[]string{"1.0.0"}[0],
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": &ContainerDefaults{},
						"1.1.0": &ContainerDefaults{},
					},
					TrackedCustomImages: map[string]CustomContainerImages{
						"1.0.0": {
							ContainerTemplate: ContainerTemplate{
								KeystoneAPIImage: &keystoneImage,
							},
							CinderVolumeImages: map[string]*string{
								"backend1": &cinderImage,
							},
						},
					},
				},
			}
		})

		It("should reject update when CustomContainerImages are unchanged and no skip annotation", func() {
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("CustomContainerImages must be updated when changing targetVersion"))
		})

		It("should allow update when skip annotation is present", func() {
			newVersion.Annotations = map[string]string{
				"core.openstack.org/skip-custom-images-validation": "true",
			}
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow update when skip annotation exists with any value", func() {
			newVersion.Annotations = map[string]string{
				"core.openstack.org/skip-custom-images-validation": "",
			}
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow update when CustomContainerImages are actually changed", func() {
			newKeystoneImage := "registry.example.com/keystone:v2.0.0"
			newVersion.Spec.CustomContainerImages.ContainerTemplate.KeystoneAPIImage = &newKeystoneImage

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow update when there are no custom images configured", func() {
			newVersion.Spec.CustomContainerImages = CustomContainerImages{}
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow update when DeployedVersion is nil (fresh install)", func() {
			oldVersion.Status.DeployedVersion = nil
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow update when target version is not changing", func() {
			newVersion.Spec.TargetVersion = "1.0.0" // Same as old version
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle edge case where TrackedCustomImages is nil", func() {
			newVersion.Status.TrackedCustomImages = nil
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle edge case where previous version not found in TrackedCustomImages", func() {
			newVersion.Status.TrackedCustomImages = map[string]CustomContainerImages{
				"0.9.0": {}, // Different version than oldVersion.Spec.TargetVersion
			}
			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle invalid old object type gracefully", func() {
			invalidOld := &DummyObject{} // Wrong type
			_, err := newVersion.ValidateUpdate(context.Background(), invalidOld, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to convert old object to OpenStackVersion"))
		})
	})
})
