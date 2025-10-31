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

package v1beta1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var openstackdataplanedeploymentlog = logf.Log.WithName("openstackdataplanedeployment-resource")

// SetupWebhookWithManager sets up the webhook with the Manager
func (r *OpenStackDataPlaneDeployment) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(r).Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment,mutating=true,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanedeployments,verbs=create;update,versions=v1beta1,name=mopenstackdataplanedeployment.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackDataPlaneDeployment{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackDataPlaneDeployment) Default() {

	openstackdataplanedeploymentlog.Info("default", "name", r.Name)
	r.Spec.Default()
}

// Default - set defaults for this OpenStackDataPlaneDeployment
func (spec *OpenStackDataPlaneDeploymentSpec) Default() {

}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment,mutating=false,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanedeployments,verbs=create;update,versions=v1beta1,name=vopenstackdataplanedeployment.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackDataPlaneDeployment{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDataPlaneDeployment) ValidateCreate() (admission.Warnings, error) {

	openstackdataplanedeploymentlog.Info("validate create", "name", r.Name)

	errors := r.Spec.ValidateCreate()
	if len(errors) != 0 {
		openstackdataplanedeploymentlog.Info("validation failed", "name", r.Name)

		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneDeployment"},
			r.Name,
			errors)
	}

	return nil, nil
}

// ValidateCreate validates the OpenStackDataPlaneDeploymentSpec on creation
func (spec *OpenStackDataPlaneDeploymentSpec) ValidateCreate() field.ErrorList {
	// TODO(user): fill in your validation logic upon object creation.

	return field.ErrorList{}
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDataPlaneDeployment) ValidateUpdate(original runtime.Object) (admission.Warnings, error) {
	openstackdataplanedeploymentlog.Info("validate update", "name", r.Name)

	errors := r.Spec.ValidateUpdate()

	if len(errors) != 0 {
		openstackdataplanedeploymentlog.Info("validation failed", "name", r.Name)

		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneDeployment"},
			r.Name,
			errors)
	}

	return nil, nil
}

// ValidateUpdate validates the OpenStackDataPlaneDeploymentSpec on update
func (spec *OpenStackDataPlaneDeploymentSpec) ValidateUpdate() field.ErrorList {
	// TODO(user): fill in your validation logic upon object update.

	return field.ErrorList{}
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackDataPlaneDeployment) ValidateDelete() (admission.Warnings, error) {
	openstackdataplanedeploymentlog.Info("validate delete", "name", r.Name)

	errors := r.Spec.ValidateDelete()

	if len(errors) != 0 {
		openstackdataplanedeploymentlog.Info("validation failed", "name", r.Name)

		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneDeployment"},
			r.Name,
			errors)
	}
	return nil, nil
}

// ValidateDelete validates the OpenStackDataPlaneDeploymentSpec on delete
func (spec *OpenStackDataPlaneDeploymentSpec) ValidateDelete() field.ErrorList {
	// TODO(user): fill in your validation logic upon object creation.

	return field.ErrorList{}
}
