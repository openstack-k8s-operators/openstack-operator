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
