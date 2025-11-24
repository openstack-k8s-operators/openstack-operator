/*
Copyright 2025.

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

// Package v1beta1 implements webhook handlers for the dataplane.openstack.org API group.
package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	dataplanev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var openstackdataplanedeploymentlog = logf.Log.WithName("openstackdataplanedeployment-resource")

// SetupOpenStackDataPlaneDeploymentWebhookWithManager registers the webhook for OpenStackDataPlaneDeployment in the manager.
func SetupOpenStackDataPlaneDeploymentWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&dataplanev1beta1.OpenStackDataPlaneDeployment{}).
		WithValidator(&OpenStackDataPlaneDeploymentCustomValidator{}).
		WithDefaulter(&OpenStackDataPlaneDeploymentCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment,mutating=true,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanedeployments,verbs=create;update,versions=v1beta1,name=mopenstackdataplanedeployment-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneDeploymentCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackDataPlaneDeployment when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneDeploymentCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &OpenStackDataPlaneDeploymentCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackDataPlaneDeployment.
func (d *OpenStackDataPlaneDeploymentCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackdataplanedeployment, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneDeployment)

	if !ok {
		return fmt.Errorf("expected an OpenStackDataPlaneDeployment object but got %T", obj)
	}
	openstackdataplanedeploymentlog.Info("Defaulting for OpenStackDataPlaneDeployment", "name", openstackdataplanedeployment.GetName())

	// Call the Default method on the OpenStackDataPlaneDeployment type
	openstackdataplanedeployment.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment,mutating=false,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanedeployments,verbs=create;update,versions=v1beta1,name=vopenstackdataplanedeployment-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneDeploymentCustomValidator struct is responsible for validating the OpenStackDataPlaneDeployment resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneDeploymentCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &OpenStackDataPlaneDeploymentCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneDeployment.
func (v *OpenStackDataPlaneDeploymentCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplanedeployment, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneDeployment)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneDeployment object but got %T", obj)
	}
	openstackdataplanedeploymentlog.Info("Validation for OpenStackDataPlaneDeployment upon creation", "name", openstackdataplanedeployment.GetName())

	// Call the ValidateCreate method on the OpenStackDataPlaneDeployment type
	return openstackdataplanedeployment.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneDeployment.
func (v *OpenStackDataPlaneDeploymentCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackdataplanedeployment, ok := newObj.(*dataplanev1beta1.OpenStackDataPlaneDeployment)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneDeployment object for the newObj but got %T", newObj)
	}
	openstackdataplanedeploymentlog.Info("Validation for OpenStackDataPlaneDeployment upon update", "name", openstackdataplanedeployment.GetName())

	// Call the ValidateUpdate method on the OpenStackDataPlaneDeployment type
	return openstackdataplanedeployment.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneDeployment.
func (v *OpenStackDataPlaneDeploymentCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplanedeployment, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneDeployment)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneDeployment object but got %T", obj)
	}
	openstackdataplanedeploymentlog.Info("Validation for OpenStackDataPlaneDeployment upon deletion", "name", openstackdataplanedeployment.GetName())

	// Call the ValidateDelete method on the OpenStackDataPlaneDeployment type
	return openstackdataplanedeployment.ValidateDelete()
}
