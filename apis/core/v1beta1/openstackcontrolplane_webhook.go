/*
Copyright 2022.

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

package v1beta1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// OpenStackControlPlaneDefaults -
type OpenStackControlPlaneDefaults struct {
	RabbitMqImageURL string
}

var openstackControlPlaneDefaults OpenStackControlPlaneDefaults

// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

// SetupOpenStackControlPlaneDefaults - initialize OpenStackControlPlane spec defaults for use with internal webhooks
func SetupOpenStackControlPlaneDefaults(defaults OpenStackControlPlaneDefaults) {
	openstackControlPlaneDefaults = defaults
	openstackcontrolplanelog.Info("OpenStackControlPlane defaults initialized", "defaults", defaults)
}

// SetupWebhookWithManager sets up the Webhook with the Manager.
func (r *OpenStackControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=Fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() error {
	openstackcontrolplanelog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.ValidateCreateServices(basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) error {
	openstackcontrolplanelog.Info("validate update", "name", r.Name)

	oldControlPlane, ok := old.(*OpenStackControlPlane)
	if !ok || oldControlPlane == nil {
		return apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.ValidateUpdateServices(oldControlPlane.Spec, basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() error {
	openstackcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}

// checkDepsEnabled - returns a non-empty string if required services are missing (disabled) for "name" service
func (r *OpenStackControlPlane) checkDepsEnabled(name string) string {

	// "msg" will hold any dependency validation error we might find
	msg := ""
	// "reqs" will be set to the required services for "name" service
	// if any of those required services are improperly disabled/missing
	reqs := ""

	switch name {
	case "Keystone":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled) {
			reqs = "MariaDB or Galera, Memcached"
		}
	case "Glance":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Memcached, Keystone"
		}
	case "Cinder":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Memcached, RabbitMQ, Keystone"
		}
	case "Placement":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Memcached, Keystone"
		}
	case "Neutron":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, RabbitMQ, Keystone"
		}
	case "Nova":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Placement.Enabled && r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled) {
			reqs = "MariaDB or Galera, Memcached, RabbitMQ, Keystone, Glance Neutron, Placement"
		}
	case "Heat":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Memcached, RabbitMQ, Keystone"
		}
	case "Swift":
		if !(r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Memcached, Keystone"
		}
	case "Horizon":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Memcached, Keystone"
		}
	case "Octavia":
		// TODO(beagles): So far we haven't declared Redis as dependency for Octavia, but we might.
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled && r.Spec.Nova.Enabled &&
			r.Spec.Ovn.Enabled) {
			reqs = "MariaDB or Galera, Memcached, RabbitMQ, Keystone, Glance, Neutron, Nova, OVN"
		}
	}

	// If "reqs" is not the empty string, we have missing requirements
	if reqs != "" {
		msg = fmt.Sprintf("%s requires these services to be enabled: %s.", name, reqs)
	}

	return msg
}

// ValidateCreateServices validating service definitions during the OpenstackControlPlane CR creation
func (r *OpenStackControlPlane) ValidateCreateServices(basePath *field.Path) field.ErrorList {
	var errors field.ErrorList

	errors = append(errors, r.ValidateServiceDependencies(basePath)...)

	// Call internal validation logic for individual service operators
	if r.Spec.Ironic.Enabled {
		errors = append(errors, r.Spec.Ironic.Template.ValidateCreate(basePath.Child("ironic").Child("template"))...)
	}

	if r.Spec.Nova.Enabled {
		errors = append(errors, r.Spec.Nova.Template.ValidateCreate(basePath.Child("nova").Child("template"))...)
	}

	return errors
}

// ValidateUpdateServices validating service definitions during the OpenstackControlPlane CR update
func (r *OpenStackControlPlane) ValidateUpdateServices(old OpenStackControlPlaneSpec, basePath *field.Path) field.ErrorList {
	var errors field.ErrorList

	errors = append(errors, r.ValidateServiceDependencies(basePath)...)

	// Call internal validation logic for individual service operators
	if r.Spec.Ironic.Enabled {
		errors = append(errors, r.Spec.Ironic.Template.ValidateUpdate(old.Ironic.Template, basePath.Child("ironic").Child("template"))...)
	}

	if r.Spec.Nova.Enabled {
		errors = append(errors, r.Spec.Nova.Template.ValidateUpdate(old.Nova.Template, basePath.Child("nova").Child("template"))...)
	}

	return errors
}

// ValidateServiceDependencies ensures that when a service is enabled then all the services it depends on are also
// enabled
func (r *OpenStackControlPlane) ValidateServiceDependencies(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Temporary check until MariaDB is deprecated
	if r.Spec.Mariadb.Enabled && r.Spec.Galera.Enabled {
		err := field.Invalid(basePath.Child("mariaDB").Child("enabled"), r.Spec.Mariadb.Enabled, "Mariadb and Galera are mutually exclusive")
		allErrs = append(allErrs, err)
	}

	// Add service dependency validations

	if r.Spec.Keystone.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Keystone"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("keystone").Child("enabled"), r.Spec.Keystone.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Glance.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Glance"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("glance").Child("enabled"), r.Spec.Glance.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Cinder.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Cinder"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("cinder").Child("enabled"), r.Spec.Cinder.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Placement.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Placement"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("placement").Child("enabled"), r.Spec.Placement.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Neutron.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Neutron"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("neutron").Child("enabled"), r.Spec.Neutron.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Nova.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Nova"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("nova").Child("enabled"), r.Spec.Nova.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Heat.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Heat"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("heat").Child("enabled"), r.Spec.Heat.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Swift.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Swift"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("swift").Child("enabled"), r.Spec.Swift.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Horizon.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Horizon"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("horizon").Child("enabled"), r.Spec.Horizon.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Octavia.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Octavia"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("octavia").Child("enabled"), r.Spec.Octavia.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}

//+kubebuilder:webhook:path=/mutate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=mopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackControlPlane) Default() {
	openstackcontrolplanelog.Info("default", "name", r.Name)

	r.DefaultServices()
}

// DefaultServices - common function for calling individual services' defaulting functions
func (r *OpenStackControlPlane) DefaultServices() {
	// RabbitMQ
	// This is a special case in that we don't own the RabbitMQ operator,
	// so we aren't able to add and call a Default function on its spec.
	// Instead we just directly set the defaults we need.
	for key, template := range r.Spec.Rabbitmq.Templates {
		if template.Image == "" {
			template.Image = openstackControlPlaneDefaults.RabbitMqImageURL
			// By-value copy, need to update
			r.Spec.Rabbitmq.Templates[key] = template
		}
	}

	// Cinder
	r.Spec.Cinder.Template.Default()

	// Galera
	for key, template := range r.Spec.Galera.Templates {
		if template.StorageClass == "" {
			template.StorageClass = r.Spec.StorageClass
		}
		if template.Secret == "" {
			template.Secret = r.Spec.Secret
		}
		template.Default()
		// By-value copy, need to update
		r.Spec.Galera.Templates[key] = template
	}

	// Glance
	r.Spec.Glance.Template.Default()

	// Ironic
	// Default Secret
	if r.Spec.Ironic.Template.Secret == "" {
		r.Spec.Ironic.Template.Secret = r.Spec.Secret
	}
	// Default DatabaseInstance
	if r.Spec.Ironic.Template.DatabaseInstance == "" {
		r.Spec.Ironic.Template.DatabaseInstance = "openstack"
	}
	// Default StorageClass
	if r.Spec.Ironic.Template.StorageClass == "" {
		r.Spec.Ironic.Template.StorageClass = r.Spec.StorageClass
	}
	r.Spec.Ironic.Template.Default()

	// Keystone
	r.Spec.Keystone.Template.Default()

	// Manila
	r.Spec.Manila.Template.Default()

	// MariaDB
	for key, template := range r.Spec.Mariadb.Templates {
		if template.StorageClass == "" {
			template.StorageClass = r.Spec.StorageClass
		}
		if template.Secret == "" {
			template.Secret = r.Spec.Secret
		}
		template.Default()
		// By-value copy, need to update
		r.Spec.Mariadb.Templates[key] = template
	}

	// Memcached
	for key, template := range r.Spec.Memcached.Templates {
		template.Default()
		// By-value copy, need to update
		r.Spec.Memcached.Templates[key] = template
	}

	// Neutron
	r.Spec.Neutron.Template.Default()

	// Nova
	r.Spec.Nova.Template.Default()

	// OVN
	for key, template := range r.Spec.Ovn.Template.OVNDBCluster {
		template.Default()
		// By-value copy, need to update
		r.Spec.Ovn.Template.OVNDBCluster[key] = template
	}

	r.Spec.Ovn.Template.OVNNorthd.Default()
	r.Spec.Ovn.Template.OVNController.Default()

	// Placement
	r.Spec.Placement.Template.Default()

	// DNS
	r.Spec.DNS.Template.Default()

	// Ceilometer
	r.Spec.Ceilometer.Template.Default()

	// Heat
	r.Spec.Heat.Template.Default()

	// Swift
	if r.Spec.Swift.Template.SwiftStorage.StorageClass == "" {
		r.Spec.Swift.Template.SwiftStorage.StorageClass = r.Spec.StorageClass
	}

	r.Spec.Swift.Template.Default()

	// Horizon
	r.Spec.Horizon.Template.Default()

	// Octavia
	r.Spec.Octavia.Template.Default()

	// Redis
	for key, template := range r.Spec.Redis.Templates {
		template.Default()
		// By-value copy, need to update
		r.Spec.Redis.Templates[key] = template
	}
}
