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

// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

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
	if err := r.ValidateServices(basePath); err != nil {
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

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.ValidateServices(basePath); err != nil {
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
		if !(r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) {
			reqs = "MariaDB or Galera"
		}
	case "Glance":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Keystone"
		}
	case "Cinder":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Keystone, RabbitMQ"
		}
	case "Placement":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Keystone"
		}
	case "Neutron":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "MariaDB or Galera, Keystone, RabbitMQ"
		}
	case "Nova":
		if !((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) && r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled &&
			r.Spec.Placement.Enabled && r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled) {
			reqs = "MariaDB or Galera, Glance, Keystone, Neutron, Placement, RabbitMQ"
		}
	}

	// If "reqs" is not the empty string, we have missing requirements
	if reqs != "" {
		msg = fmt.Sprintf("%s requires these services to be enabled: %s.", name, reqs)
	}

	return msg
}

// ValidateServices implements common function for validating services
func (r *OpenStackControlPlane) ValidateServices(basePath *field.Path) field.ErrorList {
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

	// Checks which call internal validation logic for individual service operators

	// Ironic
	if r.Spec.Ironic.Enabled {
		if err := r.Spec.Ironic.Template.ValidateCreate(basePath.Child("ironic").Child("template")); err != nil {
			allErrs = append(allErrs, err...)
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
	// Cinder
	if r.Spec.Cinder.Enabled {
		r.Spec.Cinder.Template.Default()
	}

	// Galera
	if r.Spec.Galera.Enabled {
		for key, template := range r.Spec.Galera.Templates {
			template.Default()
			// By-value copy, need to update
			r.Spec.Galera.Templates[key] = template
		}
	}

	// Glance
	if r.Spec.Glance.Enabled {
		r.Spec.Glance.Template.Default()
	}

	// Ironic
	if r.Spec.Ironic.Enabled {
		r.Spec.Ironic.Template.Default()
	}

	// Keystone
	if r.Spec.Keystone.Enabled {
		r.Spec.Keystone.Template.Default()
	}

	// Manila
	if r.Spec.Manila.Enabled {
		r.Spec.Manila.Template.Default()
	}

	// MariaDB
	if r.Spec.Mariadb.Enabled {
		for key, template := range r.Spec.Mariadb.Templates {
			template.Default()
			// By-value copy, need to update
			r.Spec.Mariadb.Templates[key] = template
		}
	}

	// Memcached
	if r.Spec.Memcached.Enabled {
		for key, template := range r.Spec.Memcached.Templates {
			template.Default()
			// By-value copy, need to update
			r.Spec.Memcached.Templates[key] = template
		}
	}

	// Neutron
	if r.Spec.Neutron.Enabled {
		r.Spec.Neutron.Template.Default()
	}

	// Nova
	if r.Spec.Nova.Enabled {
		r.Spec.Nova.Template.Default()
	}

	// OVN
	if r.Spec.Ovn.Enabled {
		for key, template := range r.Spec.Ovn.Template.OVNDBCluster {
			template.Default()
			// By-value copy, need to update
			r.Spec.Ovn.Template.OVNDBCluster[key] = template
		}

		r.Spec.Ovn.Template.OVNNorthd.Default()
	}

	// OVS
	if r.Spec.Ovs.Enabled {
		r.Spec.Ovs.Template.Default()
	}

	// Placement
	if r.Spec.Placement.Enabled {
		r.Spec.Placement.Template.Default()
	}

	// Telemetry
	if r.Spec.Telemetry.Enabled {
		r.Spec.Telemetry.Template.Default()
	}
}
