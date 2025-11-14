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
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	dataplanev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var openstackdataplanenodesetlog = logf.Log.WithName("openstackdataplanenodeset-resource")

var webhookClient client.Client

// SetupOpenStackDataPlaneNodeSetWebhookWithManager registers the webhook for OpenStackDataPlaneNodeSet in the manager.
func SetupOpenStackDataPlaneNodeSetWebhookWithManager(mgr ctrl.Manager) error {
	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).For(&dataplanev1beta1.OpenStackDataPlaneNodeSet{}).
		WithValidator(&OpenStackDataPlaneNodeSetCustomValidator{}).
		WithDefaulter(&OpenStackDataPlaneNodeSetCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-dataplane-openstack-org-v1beta1-openstackdataplanenodeset,mutating=true,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=create;update,versions=v1beta1,name=mopenstackdataplanenodeset-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneNodeSetCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackDataPlaneNodeSet when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneNodeSetCustomDefaulter struct {
}

var _ webhook.CustomDefaulter = &OpenStackDataPlaneNodeSetCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackDataPlaneNodeSet.
func (d *OpenStackDataPlaneNodeSetCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackdataplanenodeset, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneNodeSet)

	if !ok {
		return fmt.Errorf("expected an OpenStackDataPlaneNodeSet object but got %T", obj)
	}
	openstackdataplanenodesetlog.Info("Defaulting for OpenStackDataPlaneNodeSet", "name", openstackdataplanenodeset.GetName())

	// Call the Default method on the OpenStackDataPlaneNodeSet type
	openstackdataplanenodeset.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-dataplane-openstack-org-v1beta1-openstackdataplanenodeset,mutating=false,failurePolicy=fail,sideEffects=None,groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=create;update,versions=v1beta1,name=vopenstackdataplanenodeset-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackDataPlaneNodeSetCustomValidator struct is responsible for validating the OpenStackDataPlaneNodeSet resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackDataPlaneNodeSetCustomValidator struct {
}

var _ webhook.CustomValidator = &OpenStackDataPlaneNodeSetCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneNodeSet.
func (v *OpenStackDataPlaneNodeSetCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplanenodeset, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneNodeSet)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneNodeSet object but got %T", obj)
	}
	openstackdataplanenodesetlog.Info("Validation for OpenStackDataPlaneNodeSet upon creation", "name", openstackdataplanenodeset.GetName())

	// Call the ValidateCreate method on the OpenStackDataPlaneNodeSet type
	return openstackdataplanenodeset.ValidateCreate(ctx, webhookClient)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneNodeSet.
func (v *OpenStackDataPlaneNodeSetCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackdataplanenodeset, ok := newObj.(*dataplanev1beta1.OpenStackDataPlaneNodeSet)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneNodeSet object for the newObj but got %T", newObj)
	}
	openstackdataplanenodesetlog.Info("Validation for OpenStackDataPlaneNodeSet upon update", "name", openstackdataplanenodeset.GetName())

	// Call the ValidateUpdate method on the OpenStackDataPlaneNodeSet type
	return openstackdataplanenodeset.ValidateUpdate(ctx, oldObj, webhookClient)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackDataPlaneNodeSet.
func (v *OpenStackDataPlaneNodeSetCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackdataplanenodeset, ok := obj.(*dataplanev1beta1.OpenStackDataPlaneNodeSet)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackDataPlaneNodeSet object but got %T", obj)
	}
	openstackdataplanenodesetlog.Info("Validation for OpenStackDataPlaneNodeSet upon deletion", "name", openstackdataplanenodeset.GetName())

	// Call the ValidateDelete method on the OpenStackDataPlaneNodeSet type
	return openstackdataplanenodeset.ValidateDelete(ctx, webhookClient)
}
