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

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var openstackversionlog = logf.Log.WithName("openstackversion-resource")

var versionWebhookClient client.Client

// SetupOpenStackVersionWebhookWithManager registers the webhook for OpenStackVersion in the manager.
func SetupOpenStackVersionWebhookWithManager(mgr ctrl.Manager) error {
	if versionWebhookClient == nil {
		versionWebhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).For(&corev1beta1.OpenStackVersion{}).
		WithValidator(&OpenStackVersionCustomValidator{}).
		WithDefaulter(&OpenStackVersionCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-core-openstack-org-v1beta1-openstackversion,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackversions,verbs=create;update,versions=v1beta1,name=mopenstackversion-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackVersionCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackVersion when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackVersionCustomDefaulter struct {
}

var _ webhook.CustomDefaulter = &OpenStackVersionCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackVersion.
func (d *OpenStackVersionCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackversion, ok := obj.(*corev1beta1.OpenStackVersion)

	if !ok {
		return fmt.Errorf("expected an OpenStackVersion object but got %T", obj)
	}
	openstackversionlog.Info("Defaulting for OpenStackVersion", "name", openstackversion.GetName())

	// Call the Default method on the OpenStackVersion type
	openstackversion.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackversion,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackversions,verbs=create;update,versions=v1beta1,name=vopenstackversion-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackVersionCustomValidator struct is responsible for validating the OpenStackVersion resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackVersionCustomValidator struct {
}

var _ webhook.CustomValidator = &OpenStackVersionCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackVersion.
func (v *OpenStackVersionCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackversion, ok := obj.(*corev1beta1.OpenStackVersion)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackVersion object but got %T", obj)
	}
	openstackversionlog.Info("Validation for OpenStackVersion upon creation", "name", openstackversion.GetName())

	// Call the ValidateCreate method on the OpenStackVersion type
	return openstackversion.ValidateCreate(ctx, versionWebhookClient)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackVersion.
func (v *OpenStackVersionCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackversion, ok := newObj.(*corev1beta1.OpenStackVersion)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackVersion object for the newObj but got %T", newObj)
	}
	openstackversionlog.Info("Validation for OpenStackVersion upon update", "name", openstackversion.GetName())

	// Call the ValidateUpdate method on the OpenStackVersion type
	return openstackversion.ValidateUpdate(ctx, oldObj, versionWebhookClient)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackVersion.
func (v *OpenStackVersionCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackversion, ok := obj.(*corev1beta1.OpenStackVersion)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackVersion object but got %T", obj)
	}
	openstackversionlog.Info("Validation for OpenStackVersion upon deletion", "name", openstackversion.GetName())

	// Call the ValidateDelete method on the OpenStackVersion type
	return openstackversion.ValidateDelete(ctx, versionWebhookClient)
}
