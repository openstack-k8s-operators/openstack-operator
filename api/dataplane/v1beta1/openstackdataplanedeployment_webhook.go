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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var openstackdataplanedeploymentlog = logf.Log.WithName("openstackdataplanedeployment-resource")

// Default sets default values for the OpenStackDataPlaneDeployment
func (r *OpenStackDataPlaneDeployment) Default() {

	openstackdataplanedeploymentlog.Info("default", "name", r.Name)
	r.Spec.Default()
}

// Default - set defaults for this OpenStackDataPlaneDeployment
func (spec *OpenStackDataPlaneDeploymentSpec) Default() {
	if spec.ServicesOverride == nil {
		spec.ServicesOverride = []string{}
	}
}

// ValidateCreate validates the OpenStackDataPlaneDeployment on creation
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

	oldDeployment, ok := original.(*OpenStackDataPlaneDeployment)
	if !ok {
		return nil, apierrors.NewInternalError(field.InternalError(
			field.NewPath("spec"),
			fmt.Errorf("expected OpenStackDataPlaneDeployment, got %T", original)))
	}

	errors := r.Spec.ValidateUpdate(oldDeployment.Spec)

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
func (spec *OpenStackDataPlaneDeploymentSpec) ValidateUpdate(old OpenStackDataPlaneDeploymentSpec) field.ErrorList {
	newCopy := *spec
	newCopy.Default()
	old.Default()

	if !reflect.DeepEqual(newCopy, old) {
		return field.ErrorList{
			field.Invalid(
				field.NewPath("spec"),
				"object",
				"OpenStackDataPlaneDeployment Spec is immutable",
			),
		}
	}

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
