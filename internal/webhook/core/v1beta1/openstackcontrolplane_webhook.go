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

// Package v1beta1 implements webhook handlers for the core.openstack.org API group.
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

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

var ctlplaneWebhookClient client.Client

// SetupOpenStackControlPlaneWebhookWithManager registers the webhook for OpenStackControlPlane in the manager.
func SetupOpenStackControlPlaneWebhookWithManager(mgr ctrl.Manager) error {
	if ctlplaneWebhookClient == nil {
		ctlplaneWebhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).For(&corev1beta1.OpenStackControlPlane{}).
		WithValidator(&OpenStackControlPlaneCustomValidator{}).
		WithDefaulter(&OpenStackControlPlaneCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=mopenstackcontrolplane-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackControlPlaneCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackControlPlane when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackControlPlaneCustomDefaulter struct {
}

var _ webhook.CustomDefaulter = &OpenStackControlPlaneCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackControlPlane.
func (d *OpenStackControlPlaneCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackcontrolplane, ok := obj.(*corev1beta1.OpenStackControlPlane)

	if !ok {
		return fmt.Errorf("expected an OpenStackControlPlane object but got %T", obj)
	}
	openstackcontrolplanelog.Info("Defaulting for OpenStackControlPlane", "name", openstackcontrolplane.GetName())

	// Call the Default method on the OpenStackControlPlane type
	openstackcontrolplane.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=vopenstackcontrolplane-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackControlPlaneCustomValidator struct is responsible for validating the OpenStackControlPlane resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackControlPlaneCustomValidator struct {
}

var _ webhook.CustomValidator = &OpenStackControlPlaneCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackControlPlane.
func (v *OpenStackControlPlaneCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackcontrolplane, ok := obj.(*corev1beta1.OpenStackControlPlane)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackControlPlane object but got %T", obj)
	}
	openstackcontrolplanelog.Info("Validation for OpenStackControlPlane upon creation", "name", openstackcontrolplane.GetName())

	// Call the ValidateCreate method on the OpenStackControlPlane type
	return openstackcontrolplane.ValidateCreate(ctx, ctlplaneWebhookClient)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackControlPlane.
func (v *OpenStackControlPlaneCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackcontrolplane, ok := newObj.(*corev1beta1.OpenStackControlPlane)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackControlPlane object for the newObj but got %T", newObj)
	}
	openstackcontrolplanelog.Info("Validation for OpenStackControlPlane upon update", "name", openstackcontrolplane.GetName())

	// Call the ValidateUpdate method on the OpenStackControlPlane type
	return openstackcontrolplane.ValidateUpdate(ctx, oldObj, ctlplaneWebhookClient)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackControlPlane.
func (v *OpenStackControlPlaneCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackcontrolplane, ok := obj.(*corev1beta1.OpenStackControlPlane)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackControlPlane object but got %T", obj)
	}
	openstackcontrolplanelog.Info("Validation for OpenStackControlPlane upon deletion", "name", openstackcontrolplane.GetName())

	// Call the ValidateDelete method on the OpenStackControlPlane type
	return openstackcontrolplane.ValidateDelete(ctx, ctlplaneWebhookClient)
}
