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

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=Fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() error {
	openstackcontrolplanelog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, r.ValidateServices()...)

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "openstackcontrolplane.openstack.org",
				Kind:  "OpenstackControlPlane",
			},
			r.Name, allErrs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) error {
	openstackcontrolplanelog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList

	allErrs = append(allErrs, r.ValidateServices()...)

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "openstackcontrolplane.openstack.org",
				Kind:  "OpenstackControlPlane",
			},
			r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() error {
	openstackcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *OpenStackControlPlane) checkDepsEnabled(name string) bool {
	switch name {
	case "Keystone":
		return (r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled)
	case "Glance":
		return (r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) &&
			r.Spec.Keystone.Enabled
	case "Cinder":
		return ((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) &&
			r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled)
	case "Placement":
		return (r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) &&
			r.Spec.Keystone.Enabled
	case "Neutron":
		return ((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) &&
			r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled)
	case "Nova":
		return ((r.Spec.Mariadb.Enabled || r.Spec.Galera.Enabled) &&
			r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Placement.Enabled &&
			r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled)
	case "Ironic-JsonRPC":
		return (r.Spec.Mariadb.Enabled && r.Spec.Keystone.Enabled)
	case "Ironic":
		return (r.Spec.Mariadb.Enabled && r.Spec.Keystone.Enabled &&
			r.Spec.Rabbitmq.Enabled)
	}
	return true
}

// ValidateServices implements common function for validating services
func (r *OpenStackControlPlane) ValidateServices() field.ErrorList {
	var allErrs field.ErrorList
	var path *field.Path
	var dependencies string

	// Temporary check until MariaDB is deprecated
	if r.Spec.Mariadb.Enabled && r.Spec.Galera.Enabled {
		pathA := field.NewPath("spec").Child("Mariadb").Child("enabled")
		pathB := field.NewPath("spec").Child("Galera").Child("enabled")
		allErrs = append(
			allErrs,
			field.Duplicate(
				pathA,
				fmt.Sprintf("%s - Mariadb and Galera are mutually exclusive", pathB.String()),
			),
		)
	}

	// Add service dependency validations
	errorMsg := "%s service dependencies are not enabled, needs: %s"

	if r.Spec.Keystone.Enabled && !r.checkDepsEnabled("Keystone") {
		path = field.NewPath("spec").Child("Keystone").Child("enabled")
		dependencies = "Mariadb"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Keystone.Enabled,
				fmt.Sprintf(errorMsg, "Keystone", dependencies),
			),
		)
	}

	if r.Spec.Glance.Enabled && !r.checkDepsEnabled("Glance") {
		path = field.NewPath("spec").Child("Glance").Child("enabled")
		dependencies = "Mariadb and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Glance.Enabled,
				fmt.Sprintf(errorMsg, "Glance", dependencies),
			),
		)
	}

	if r.Spec.Cinder.Enabled && !r.checkDepsEnabled("Cinder") {
		path = field.NewPath("spec").Child("Cinder").Child("enabled")
		dependencies = "Mariadb, Rabbitmq and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Cinder.Enabled,
				fmt.Sprintf(errorMsg, "Cinder", dependencies),
			),
		)
	}

	if r.Spec.Placement.Enabled && !r.checkDepsEnabled("Placement") {
		path = field.NewPath("spec").Child("Placement").Child("enabled")
		dependencies = "Mariadb and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Placement.Enabled,
				fmt.Sprintf(errorMsg, "Placement", dependencies),
			),
		)
	}

	if r.Spec.Neutron.Enabled && !r.checkDepsEnabled("Neutron") {
		path = field.NewPath("spec").Child("Neutron").Child("enabled")
		dependencies = "Mariadb, Rabbitmq and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Neutron.Enabled,
				fmt.Sprintf(errorMsg, "Neutron", dependencies),
			),
		)
	}

	if r.Spec.Nova.Enabled && !r.checkDepsEnabled("Nova") {
		path = field.NewPath("spec").Child("Nova").Child("enabled")
		dependencies = "Mariadb, Rabbitmq, Keystone, Placement, Neutron and Glance"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Nova.Enabled,
				fmt.Sprintf(errorMsg, "Nova", dependencies),
			),
		)
	}

	if r.Spec.Ironic.Enabled && !r.checkDepsEnabled("Ironic-JsonRPC") {
		path = field.NewPath("spec").Child("Ironic").Child("enabled")
		dependencies = "Mariadb and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Ironic.Enabled,
				fmt.Sprintf(errorMsg, "Ironic", dependencies),
			),
		)
	}

	if (r.Spec.Ironic.Enabled && r.Spec.Ironic.Template.RPCTransport == "oslo") && !r.checkDepsEnabled("Ironic") {
		path = field.NewPath("spec").Child("Ironic").Child("enabled")
		dependencies = "Mariadb, Rabbitmq and Keystone"
		allErrs = append(
			allErrs,
			field.Invalid(
				path,
				r.Spec.Ironic.Enabled,
				fmt.Sprintf(errorMsg, "Ironic", dependencies),
			),
		)
	}

	return allErrs
}
