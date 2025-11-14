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

// Package v1beta1 implements webhook handlers for the client.openstack.org API group.
package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clientv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/client/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var openstackclientlog = logf.Log.WithName("openstackclient-resource")

// SetupOpenStackClientWebhookWithManager registers the webhook for OpenStackClient in the manager.
func SetupOpenStackClientWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&clientv1beta1.OpenStackClient{}).
		WithValidator(&OpenStackClientCustomValidator{}).
		WithDefaulter(&OpenStackClientCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-client-openstack-org-v1beta1-openstackclient,mutating=true,failurePolicy=fail,sideEffects=None,groups=client.openstack.org,resources=openstackclients,verbs=create;update,versions=v1beta1,name=mopenstackclient-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackClientCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind OpenStackClient when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type OpenStackClientCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &OpenStackClientCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind OpenStackClient.
func (d *OpenStackClientCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackclient, ok := obj.(*clientv1beta1.OpenStackClient)

	if !ok {
		return fmt.Errorf("expected an OpenStackClient object but got %T", obj)
	}
	openstackclientlog.Info("Defaulting for OpenStackClient", "name", openstackclient.GetName())

	// Call the Default method on the OpenStackClient type
	openstackclient.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-client-openstack-org-v1beta1-openstackclient,mutating=false,failurePolicy=fail,sideEffects=None,groups=client.openstack.org,resources=openstackclients,verbs=create;update,versions=v1beta1,name=vopenstackclient-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackClientCustomValidator struct is responsible for validating the OpenStackClient resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type OpenStackClientCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &OpenStackClientCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackClient.
func (v *OpenStackClientCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackclient, ok := obj.(*clientv1beta1.OpenStackClient)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackClient object but got %T", obj)
	}
	openstackclientlog.Info("Validation for OpenStackClient upon creation", "name", openstackclient.GetName())

	// Call the ValidateCreate method on the OpenStackClient type
	return openstackclient.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type OpenStackClient.
func (v *OpenStackClientCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackclient, ok := newObj.(*clientv1beta1.OpenStackClient)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackClient object for the newObj but got %T", newObj)
	}
	openstackclientlog.Info("Validation for OpenStackClient upon update", "name", openstackclient.GetName())

	// Call the ValidateUpdate method on the OpenStackClient type
	return openstackclient.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type OpenStackClient.
func (v *OpenStackClientCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackclient, ok := obj.(*clientv1beta1.OpenStackClient)
	if !ok {
		return nil, fmt.Errorf("expected a OpenStackClient object but got %T", obj)
	}
	openstackclientlog.Info("Validation for OpenStackClient upon deletion", "name", openstackclient.GetName())

	// Call the ValidateDelete method on the OpenStackClient type
	return openstackclient.ValidateDelete()
}
