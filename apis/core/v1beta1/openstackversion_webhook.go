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

package v1beta1

import (
	"context"
	"os"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var versionWebhookClient goClient.Client

// OpenStackVersionDefaults -
type OpenStackVersionDefaults struct {
	AvailableVersion string
}

var openstackVersionDefaults OpenStackVersionDefaults

// log is for logging in this package.
var openstackversionlog = logf.Log.WithName("openstackversion-resource")

// SetupOpenStackVersionDefaults - initialize OpenStackControlPlane spec defaults for use with internal webhooks
func SetupOpenStackVersionDefaults(defaults OpenStackVersionDefaults) {
	openstackVersionDefaults = defaults
	openstackversionlog.Info("OpenStackVersion defaults initialized", "defaults", defaults)
}

// SetupWebhookWithManager - register OpenStackVersion with the controller manager
func (r *OpenStackVersion) SetupWebhookWithManager(mgr ctrl.Manager) error {

	if versionWebhookClient == nil {
		versionWebhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-core-openstack-org-v1beta1-openstackversion,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackversions,verbs=create;update,versions=v1beta1,name=mopenstackversion.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackVersion{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackVersion) Default() {
	openstackversionlog.Info("default", "name", r.Name)
	if r.Spec.TargetVersion == "" {
		r.Spec.TargetVersion = openstackVersionDefaults.AvailableVersion
	}

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackversion,mutating=false,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackversions,verbs=create;update,versions=v1beta1,name=vopenstackversion.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackVersion{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackVersion) ValidateCreate() (admission.Warnings, error) {
	openstackversionlog.Info("validate create", "name", r.Name)

	if r.Spec.TargetVersion != openstackVersionDefaults.AvailableVersion {
		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "TargetVersion",
				BadValue: r.Spec.TargetVersion,
				Detail:   "Invalid value: " + r.Spec.TargetVersion + " must equal available version.",
			},
		)
	}

	versionList := &OpenStackVersionList{}

	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}

	if err := versionWebhookClient.List(context.TODO(), versionList, listOpts...); err != nil {

		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "",
				BadValue: r.Spec.TargetVersion,
				Detail:   err.Error(),
			},
		)

	}

	if len(versionList.Items) >= 1 {

		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "",
				BadValue: r.Spec.TargetVersion,
				Detail:   "Only one OpenStackVersion instance is supported at this time.",
			},
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackVersion) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	openstackversionlog.Info("validate update", "name", r.Name)

	_, ok := r.Status.ContainerImageVersionDefaults[r.Spec.TargetVersion]
	if r.Spec.TargetVersion != openstackVersionDefaults.AvailableVersion && !ok {
		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "TargetVersion",
				BadValue: r.Spec.TargetVersion,
				Detail:   "Invalid value: " + r.Spec.TargetVersion + " must be in the list of current or previous available versions.",
			},
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackVersion) ValidateDelete() (admission.Warnings, error) {
	openstackversionlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}

// SetupVersionDefaults -
func SetupVersionDefaults() {
	openstackVersionDefaults := OpenStackVersionDefaults{
		AvailableVersion: GetOpenStackReleaseVersion(os.Environ()),
	}

	SetupOpenStackVersionDefaults(openstackVersionDefaults)
}
