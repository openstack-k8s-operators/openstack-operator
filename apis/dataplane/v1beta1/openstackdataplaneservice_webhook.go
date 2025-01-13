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

var openstackdataplaneservicelog = logf.Log.WithName("openstackdataplaneservice-resource")

// SetupWebhookWithManager sets up the webhook with the Manager
func (r *OpenStackDataPlaneService) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(r).Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-dataplane-openstack-org-v1beta1-openstackdataplaneservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=create;update,versions=v1beta1,name=mopenstackdataplaneservice.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackDataPlaneService{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackDataPlaneService) Default() {

	openstackdataplaneservicelog.Info("default", "name", r.Name)
	r.DefaultLabels()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-dataplane-openstack-org-v1beta1-openstackdataplaneservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=create;update,versions=v1beta1,name=vopenstackdataplaneservice.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackDataPlaneService{}

func (r *OpenStackDataPlaneService) ValidateCreate() (admission.Warnings, error) {

	openstackdataplaneservicelog.Info("validate create", "name", r.Name)

	errors := r.Spec.ValidateCreate()

	if len(errors) != 0 {
		openstackdataplaneservicelog.Info("validation failed", "name", r.Name)
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneService"},
			r.Name,
			errors,
		)
	}

	return nil, nil
}

func (r *OpenStackDataPlaneServiceSpec) ValidateArtifact() field.ErrorList {
	if len(r.Playbook) == len(r.PlaybookContents) && len(r.Playbook) == len(r.Role) && len(r.Playbook) == 0 {
		return field.ErrorList{
			field.Invalid(
				field.NewPath("Playbook"),
				r.Playbook, "Playbook, PlaybookContents and Role cannot be empty at the same time",
			),
			field.Invalid(
				field.NewPath("PlaybookContents"),
				r.Playbook, "Playbook, PlaybookContents and Role cannot be empty at the same time",
			),
			field.Invalid(
				field.NewPath("Role"),
				r.Playbook, "Playbook, PlaybookContents and Role cannot be empty at the same time",
			),
		}
	}

	return field.ErrorList{}
}

func (r *OpenStackDataPlaneServiceSpec) ValidateCreate() field.ErrorList {
	return r.ValidateArtifact()
}

func (r *OpenStackDataPlaneService) ValidateUpdate(original runtime.Object) (admission.Warnings, error) {
	openstackdataplaneservicelog.Info("validate update", "name", r.Name)
	errors := r.Spec.ValidateUpdate()

	if len(errors) != 0 {
		openstackdataplaneservicelog.Info("validation failed", "name", r.Name)
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneService"},
			r.Name,
			errors,
		)
	}
	return nil, nil
}

func (r *OpenStackDataPlaneServiceSpec) ValidateUpdate() field.ErrorList {
	return r.ValidateArtifact()
}

func (r *OpenStackDataPlaneService) ValidateDelete() (admission.Warnings, error) {
	openstackdataplaneservicelog.Info("validate delete", "name", r.Name)

	errors := r.Spec.ValidateDelete()

	if len(errors) != 0 {
		openstackdataplaneservicelog.Info("validation failed", "name", r.Name)
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneService"},
			r.Name,
			errors,
		)
	}
	return nil, nil
}

func (r *OpenStackDataPlaneServiceSpec) ValidateDelete() field.ErrorList {
	// TODO(user): fill in your validation logic upon object creation.

	return field.ErrorList{}
}
