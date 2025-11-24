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
var openstackdataplaneservicelog = logf.Log.WithName("openstackdataplaneservice-resource")

// SetupOpenStackDataPlaneServiceWebhookWithManager registers the webhook for OpenStackDataPlaneService in the manager.
func SetupOpenStackDataPlaneServiceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&dataplanev1beta1.OpenStackDataPlaneService{}).
		WithValidator(&OpenStackDataPlaneServiceCustomValidator{}).
		WithDefaulter(&OpenStackDataPlaneServiceCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-dataplane-openstack-org-v1beta1-openstackdataplaneservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=create;update,versions=v1beta1,name=mopenstackdataplaneservice-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneServiceCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackDataPlaneService when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneServiceCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &OpenStackDataPlaneServiceCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackDataPlaneService.
func (d *OpenStackDataPlaneServiceCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackdataplaneservice, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneService)

	if !ok {
		return fmt.Errorf("expected an OpenStackDataPlaneService object but got %T", obj)
	}
	openstackdataplaneservicelog.Info("Defaulting for OpenStackDataPlaneService", "name", openstackdataplaneservice.GetName())

	// Call the Default method on the OpenStackDataPlaneService type
	openstackdataplaneservice.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dataplane-openstack-org-v1beta1-openstackdataplaneservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=create;update,versions=v1beta1,name=vopenstackdataplaneservice-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneServiceCustomValidator struct is responsible for validating the OpenStackDataPlaneService resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneServiceCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &OpenStackDataPlaneServiceCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneService.
func (v *OpenStackDataPlaneServiceCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplaneservice, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneService)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneService object but got %T", obj)
	}
	openstackdataplaneservicelog.Info("Validation for OpenStackDataPlaneService upon creation", "name", openstackdataplaneservice.GetName())

	// Call the ValidateCreate method on the OpenStackDataPlaneService type
	return openstackdataplaneservice.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneService.
func (v *OpenStackDataPlaneServiceCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackdataplaneservice, ok := newObj.(*dataplanev1beta1.OpenStackDataPlaneService)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneService object for the newObj but got %T", newObj)
	}
	openstackdataplaneservicelog.Info("Validation for OpenStackDataPlaneService upon update", "name", openstackdataplaneservice.GetName())

	// Call the ValidateUpdate method on the OpenStackDataPlaneService type
	return openstackdataplaneservice.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneService.
func (v *OpenStackDataPlaneServiceCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplaneservice, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneService)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneService object but got %T", obj)
	}
	openstackdataplaneservicelog.Info("Validation for OpenStackDataPlaneService upon deletion", "name", openstackdataplaneservice.GetName())

	// Call the ValidateDelete method on the OpenStackDataPlaneService type
	return openstackdataplaneservice.ValidateDelete()
}
