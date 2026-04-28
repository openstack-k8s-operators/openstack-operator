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

// Package v1beta1 implements webhook handlers for the assistant.openstack.org API group.
package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	assistantv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
)

// nolint:unused
var openstackassistantlog = logf.Log.WithName("openstackassistant-resource")

// SetupOpenStackAssistantWebhookWithManager registers the webhook for OpenStackAssistant in the manager.
func SetupOpenStackAssistantWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&assistantv1beta1.OpenStackAssistant{}).
		WithValidator(&OpenStackAssistantCustomValidator{}).
		WithDefaulter(&OpenStackAssistantCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-assistant-openstack-org-v1beta1-openstackassistant,mutating=true,failurePolicy=fail,sideEffects=None,groups=assistant.openstack.org,resources=openstackassistants,verbs=create;update,versions=v1beta1,name=mopenstackassistant-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackAssistantCustomDefaulter struct is responsible for setting default values on the custom resource.
type OpenStackAssistantCustomDefaulter struct{}

var _ webhook.CustomDefaulter = &OpenStackAssistantCustomDefaulter{}

// Default implements webhook.CustomDefaulter
func (d *OpenStackAssistantCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	openstackassistant, ok := obj.(*assistantv1beta1.OpenStackAssistant)
	if !ok {
		return fmt.Errorf("expected an OpenStackAssistant object but got %T", obj)
	}
	openstackassistantlog.Info("Defaulting for OpenStackAssistant", "name", openstackassistant.GetName())

	openstackassistant.Default()

	return nil
}

// +kubebuilder:webhook:path=/validate-assistant-openstack-org-v1beta1-openstackassistant,mutating=false,failurePolicy=fail,sideEffects=None,groups=assistant.openstack.org,resources=openstackassistants,verbs=create;update,versions=v1beta1,name=vopenstackassistant-v1beta1.kb.io,admissionReviewVersions=v1

// OpenStackAssistantCustomValidator struct is responsible for validating the OpenStackAssistant resource.
type OpenStackAssistantCustomValidator struct{}

var _ webhook.CustomValidator = &OpenStackAssistantCustomValidator{}

// ValidateCreate implements webhook.CustomValidator
func (v *OpenStackAssistantCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackassistant, ok := obj.(*assistantv1beta1.OpenStackAssistant)
	if !ok {
		return nil, fmt.Errorf("expected an OpenStackAssistant object but got %T", obj)
	}
	openstackassistantlog.Info("Validation for OpenStackAssistant upon creation", "name", openstackassistant.GetName())

	return openstackassistant.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator
func (v *OpenStackAssistantCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	openstackassistant, ok := newObj.(*assistantv1beta1.OpenStackAssistant)
	if !ok {
		return nil, fmt.Errorf("expected an OpenStackAssistant object for the newObj but got %T", newObj)
	}
	openstackassistantlog.Info("Validation for OpenStackAssistant upon update", "name", openstackassistant.GetName())

	return openstackassistant.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator
func (v *OpenStackAssistantCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	openstackassistant, ok := obj.(*assistantv1beta1.OpenStackAssistant)
	if !ok {
		return nil, fmt.Errorf("expected an OpenStackAssistant object but got %T", obj)
	}
	openstackassistantlog.Info("Validation for OpenStackAssistant upon deletion", "name", openstackassistant.GetName())

	return openstackassistant.ValidateDelete()
}
