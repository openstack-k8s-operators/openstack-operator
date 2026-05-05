package v1beta1

import (
	"context"
	"fmt"

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

	Context("MinorUpdateTargetStageAnnotation validation", func() {

		BeforeEach(func() {
			SetupOpenStackVersionDefaults(OpenStackVersionDefaults{
				AvailableVersion: "1.1.0",
			})
		})

		It("should reject update when annotation value is invalid", func() {
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations = map[string]string{
				MinorUpdateTargetStageAnnotation: "tyop",
			}

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`Invalid target stage "tyop"`))
			Expect(err.Error()).To(ContainSubstring("Must be one of: " + MinorUpdateStageOVNControlplane))
		})

		It("should reject update when annotation is present but empty", func() {
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations = map[string]string{
				MinorUpdateTargetStageAnnotation: "",
			}

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Annotation value must not be empty"))
		})

		It("should allow update when annotation is a valid stage", func() {
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations = map[string]string{
				MinorUpdateTargetStageAnnotation: MinorUpdateStageRabbitMQ,
			}

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject moving target stage backward during minor update", func() {
			deployed := "1.0.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: MinorUpdateStageRabbitMQ,
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": {},
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations[MinorUpdateTargetStageAnnotation] = MinorUpdateStageOVNDataplane

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Cannot move update target stage"))
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf(`from %q to earlier stage %q while minor update is in progress`, MinorUpdateStageRabbitMQ, MinorUpdateStageOVNDataplane)))
		})

		It("should allow advancing target stage during minor update", func() {
			deployed := "1.0.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: MinorUpdateStageOVNControlplane,
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": {},
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations[MinorUpdateTargetStageAnnotation] = MinorUpdateStageOVNDataplane

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow removing target stage annotation during minor update", func() {
			deployed := "1.0.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: MinorUpdateStageKeystone,
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": {},
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			delete(newVersion.Annotations, MinorUpdateTargetStageAnnotation)

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow moving target stage backward when minor update is not in progress for preparation to update", func() {
			deployed := "1.1.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: MinorUpdateStageRabbitMQ,
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.1.0": {},
					},
				},
			}
			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations[MinorUpdateTargetStageAnnotation] = MinorUpdateStageOVNControlplane

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject adding target stage behind completed progress during minor update", func() {
			deployed := "1.0.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": {},
						"1.1.0": {},
					},
				},
			}
			oldVersion.Status.Conditions.MarkTrue(
				OpenStackVersionMinorUpdateOVNControlplane,
				OpenStackVersionMinorUpdateReadyMessage,
			)
			oldVersion.Status.Conditions.MarkTrue(
				OpenStackVersionMinorUpdateOVNDataplane,
				OpenStackVersionMinorUpdateReadyMessage,
			)

			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations = map[string]string{
				MinorUpdateTargetStageAnnotation: MinorUpdateStageOVNControlplane,
			}

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Cannot set update target stage"))
			Expect(err.Error()).To(ContainSubstring(MinorUpdateStageOVNControlplane))
			Expect(err.Error()).To(ContainSubstring(MinorUpdateStageOVNDataplane))
		})

		It("should allow adding target stage at current progress during minor update", func() {
			deployed := "1.0.0"
			oldVersion := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
				Status: OpenStackVersionStatus{
					DeployedVersion: &deployed,
					ContainerImageVersionDefaults: map[string]*ContainerDefaults{
						"1.0.0": {},
						"1.1.0": {},
					},
				},
			}
			oldVersion.Status.Conditions.MarkTrue(
				OpenStackVersionMinorUpdateOVNControlplane,
				OpenStackVersionMinorUpdateReadyMessage,
			)

			newVersion := oldVersion.DeepCopy()
			newVersion.Annotations = map[string]string{
				MinorUpdateTargetStageAnnotation: MinorUpdateStageOVNControlplane,
			}

			_, err := newVersion.ValidateUpdate(context.Background(), oldVersion, nil)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("ValidateCreate MinorUpdateTargetStageAnnotation validation", func() {

		BeforeEach(func() {
			SetupOpenStackVersionDefaults(OpenStackVersionDefaults{
				AvailableVersion: "1.1.0",
			})
		})

		It("should reject create when annotation value is invalid", func() {
			version := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: "tyop",
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
			}

			_, err := version.ValidateCreate(context.Background(), nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`Invalid target stage "tyop"`))
		})

		It("should reject create when annotation is present but empty", func() {
			version := &OpenStackVersion{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-version",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						MinorUpdateTargetStageAnnotation: "",
					},
				},
				Spec: OpenStackVersionSpec{TargetVersion: "1.1.0"},
			}

			_, err := version.ValidateCreate(context.Background(), nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Annotation value must not be empty"))
		})
	})
})
