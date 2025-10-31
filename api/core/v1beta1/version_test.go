package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenStackReleaseVersion", func() {

	Context("test interal getOpenStackReleaseVersion", func() {

		// NOTE: this is the default behavior where OPENSTACK_RELEASE_VERSION is just an environment variable
		// and enables the clearest understanding of the version number with regards to testing upgrades
		It("Generates a default version based on the OPENSTACK_RELEASE_VERSION when no mode is set", func() {
			Expect(
				getOpenStackReleaseVersion("1.2.3", "", "openstack-operator.v1.0.0-0.1724144685.p"),
			).To(Equal("1.2.3"))

		})

		It("Generates a default version based on the OPENSTACK_RELEASE_VERSION when invalid mode is set", func() {
			Expect(
				getOpenStackReleaseVersion("1.2.3", "asdf", "openstack-operator.v1.0.0-0.1724144685.p"),
			).To(Equal("1.2.3"))

		})

		// NOTE: this is what some downstream projects use for custom release automation
		// Will envolve extra understanding of the version number when testing upgrades with regards to how the
		// epoch gets appended
		It("Generates a version which appends the epoch when csvEpochAppend is enabled", func() {
			Expect(
				getOpenStackReleaseVersion("1.2.3", "csvEpochAppend", "openstack-operator.v1.0.0-0.1234567890.p"),
			).To(Equal("1.2.3.1234567890.p"))

		})

	})

})

var _ = Describe("customContainerImagesModified", func() {

	Context("CinderVolumeImages comparison", func() {

		It("should return true when both CinderVolumeImages are nil", func() {
			a := CustomContainerImages{}
			b := CustomContainerImages{}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return true when both CinderVolumeImages are empty", func() {
			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return true when CinderVolumeImages have same key-value pairs", func() {
			backend1Image := "registry.example.com/cinder-volume:backend1-v1.0.0"
			backend2Image := "registry.example.com/cinder-volume:backend2-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
					"backend2": &backend2Image,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
					"backend2": &backend2Image,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return false when one CinderVolumeImages is nil and other is not", func() {
			backend1Image := "registry.example.com/cinder-volume:backend1-v1.0.0"

			a := CustomContainerImages{}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when CinderVolumeImages have different number of backends", func() {
			backend1Image := "registry.example.com/cinder-volume:backend1-v1.0.0"
			backend2Image := "registry.example.com/cinder-volume:backend2-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
					"backend2": &backend2Image,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when CinderVolumeImages have different backend names", func() {
			backend1Image := "registry.example.com/cinder-volume:backend1-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend2": &backend1Image,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when CinderVolumeImages have different image values", func() {
			backend1ImageV1 := "registry.example.com/cinder-volume:backend1-v1.0.0"
			backend1ImageV2 := "registry.example.com/cinder-volume:backend1-v2.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1ImageV1,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1ImageV2,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when one backend has nil value and other has a value", func() {
			backend1Image := "registry.example.com/cinder-volume:backend1-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": nil,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &backend1Image,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return true when both backends have nil values", func() {
			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": nil,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": nil,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

	})

	Context("ManilaShareImages comparison", func() {

		It("should return true when both ManilaShareImages are nil", func() {
			a := CustomContainerImages{}
			b := CustomContainerImages{}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return true when both ManilaShareImages are empty", func() {
			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return true when ManilaShareImages have same key-value pairs", func() {
			nfsImage := "registry.example.com/manila-share:nfs-v1.0.0"
			cephfsImage := "registry.example.com/manila-share:cephfs-v1.0.0"

			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs":    &nfsImage,
					"cephfs": &cephfsImage,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs":    &nfsImage,
					"cephfs": &cephfsImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return false when one ManilaShareImages is nil and other is not", func() {
			nfsImage := "registry.example.com/manila-share:nfs-v1.0.0"

			a := CustomContainerImages{}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &nfsImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when ManilaShareImages have different number of share backends", func() {
			nfsImage := "registry.example.com/manila-share:nfs-v1.0.0"
			cephfsImage := "registry.example.com/manila-share:cephfs-v1.0.0"

			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &nfsImage,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs":    &nfsImage,
					"cephfs": &cephfsImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when ManilaShareImages have different share backend names", func() {
			shareImage := "registry.example.com/manila-share:v1.0.0"

			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &shareImage,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"cephfs": &shareImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when ManilaShareImages have different image values", func() {
			nfsImageV1 := "registry.example.com/manila-share:nfs-v1.0.0"
			nfsImageV2 := "registry.example.com/manila-share:nfs-v2.0.0"

			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &nfsImageV1,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &nfsImageV2,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when one share backend has nil value and other has a value", func() {
			nfsImage := "registry.example.com/manila-share:nfs-v1.0.0"

			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": nil,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": &nfsImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return true when both share backends have nil values", func() {
			a := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": nil,
				},
			}
			b := CustomContainerImages{
				ManilaShareImages: map[string]*string{
					"nfs": nil,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

	})

	Context("Combined CinderVolumeImages and ManilaShareImages comparison", func() {

		It("should return true when both CinderVolumeImages and ManilaShareImages are identical", func() {
			cinderImage := "registry.example.com/cinder-volume:backend1-v1.0.0"
			manilaImage := "registry.example.com/manila-share:nfs-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

		It("should return false when CinderVolumeImages match but ManilaShareImages differ", func() {
			cinderImage := "registry.example.com/cinder-volume:backend1-v1.0.0"
			manilaImageV1 := "registry.example.com/manila-share:nfs-v1.0.0"
			manilaImageV2 := "registry.example.com/manila-share:nfs-v2.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImageV1,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImageV2,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return false when ManilaShareImages match but CinderVolumeImages differ", func() {
			cinderImageV1 := "registry.example.com/cinder-volume:backend1-v1.0.0"
			cinderImageV2 := "registry.example.com/cinder-volume:backend1-v2.0.0"
			manilaImage := "registry.example.com/manila-share:nfs-v1.0.0"

			a := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImageV1,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			b := CustomContainerImages{
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImageV2,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

	})

	Context("ContainerTemplate fields comparison", func() {

		It("should return false when ContainerTemplate fields differ but CinderVolume/Manila fields match", func() {
			keystoneImageV1 := "registry.example.com/keystone:v1.0.0"
			keystoneImageV2 := "registry.example.com/keystone:v2.0.0"
			cinderImage := "registry.example.com/cinder-volume:backend1-v1.0.0"

			a := CustomContainerImages{
				ContainerTemplate: ContainerTemplate{
					KeystoneAPIImage: &keystoneImageV1,
				},
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
			}
			b := CustomContainerImages{
				ContainerTemplate: ContainerTemplate{
					KeystoneAPIImage: &keystoneImageV2,
				},
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeTrue())
		})

		It("should return true when all fields including ContainerTemplate match", func() {
			keystoneImage := "registry.example.com/keystone:v1.0.0"
			novaImage := "registry.example.com/nova:v1.0.0"
			cinderImage := "registry.example.com/cinder-volume:backend1-v1.0.0"
			manilaImage := "registry.example.com/manila-share:nfs-v1.0.0"

			a := CustomContainerImages{
				ContainerTemplate: ContainerTemplate{
					KeystoneAPIImage: &keystoneImage,
					NovaAPIImage:     &novaImage,
				},
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			b := CustomContainerImages{
				ContainerTemplate: ContainerTemplate{
					KeystoneAPIImage: &keystoneImage,
					NovaAPIImage:     &novaImage,
				},
				CinderVolumeImages: map[string]*string{
					"backend1": &cinderImage,
				},
				ManilaShareImages: map[string]*string{
					"nfs": &manilaImage,
				},
			}
			Expect(customContainerImagesAllModified(a, b)).To(BeFalse())
		})

	})

})
