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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type OpenStackLightspeedDefaults struct {
	RAGImageURL string
}

var openStackLightspeedDefaults OpenStackLightspeedDefaults

// SetupOpenStackLightspeedDefaults - initialize OpenStackLightspeed spec defaults for use with either internal or external webhooks
func SetupOpenStackLightspeedDefaults(defaults OpenStackLightspeedDefaults) {
	openStackLightspeedDefaults = defaults
	openstacklightspeedlog.Info("OpenStackLightspeed defaults initialized", "defaults", defaults)
}

// log is for logging in this package.
var openstacklightspeedlog = logf.Log.WithName("openstacklightspeed-resource")

func (r *OpenStackLightspeed) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-lightspeed-openstack-org-v1beta1-openstacklightspeed,mutating=true,failurePolicy=fail,sideEffects=None,groups=lightspeed.openstack.org,resources=openstacklightspeeds,verbs=create;update,versions=v1beta1,name=mopenstacklightspeed.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackLightspeed{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackLightspeed) Default() {
	openstacklightspeedlog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Default - set defaults for this OpenStackLightspeed spec
func (spec *OpenStackLightspeedSpec) Default() {
	if spec.RAGImage == "" {
		spec.RAGImage = openStackLightspeedDefaults.RAGImageURL
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-lightspeed-openstack-org-v1beta1-openstacklightspeed,mutating=false,failurePolicy=fail,sideEffects=None,groups=lightspeed.openstack.org,resources=openstacklightspeeds,verbs=create;update,versions=v1beta1,name=vopenstacklightspeed.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackLightspeed{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackLightspeed) ValidateCreate() (admission.Warnings, error) {
	openstacklightspeedlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackLightspeed) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	openstacklightspeedlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackLightspeed) ValidateDelete() (admission.Warnings, error) {
	openstacklightspeedlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
