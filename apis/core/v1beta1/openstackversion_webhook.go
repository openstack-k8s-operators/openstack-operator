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
	"fmt"
	"os"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
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

	versionList, err := GetOpenStackVersions(r.Namespace, versionWebhookClient)

	if err != nil {

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

	// Validate CustomContainerImages changes during version updates
	oldVersion, ok := old.(*OpenStackVersion)
	if !ok {
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to convert old object to OpenStackVersion"))
	}

	// Check if targetVersion is changing and this is a minor update
	if oldVersion.Spec.TargetVersion != r.Spec.TargetVersion && oldVersion.Status.DeployedVersion != nil {
		// Check if the skip annotation is present
		skipValidation := false
		if r.Annotations != nil {
			if _, exists := r.Annotations["core.openstack.org/skip-custom-images-validation"]; exists {
				skipValidation = true
			}
		}

		// When changing target version during a minor update, ensure CustomContainerImages are also updated
		// unless skip annotation is present
		if !skipValidation && hasAnyCustomImage(r.Spec.CustomContainerImages) {

			// Get the tracked custom images for the previous version
			if r.Status.TrackedCustomImages != nil {
				if trackedImages, exists := r.Status.TrackedCustomImages[oldVersion.Spec.TargetVersion]; exists {
					// Compare current CustomContainerImages with tracked ones
					if !customContainerImagesAllModified(r.Spec.CustomContainerImages, trackedImages) {
						return nil, apierrors.NewForbidden(
							schema.GroupResource{
								Group:    GroupVersion.WithKind("OpenStackVersion").Group,
								Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
							}, r.GetName(), &field.Error{
								Type:     field.ErrorTypeForbidden,
								Field:    "spec.customContainerImages",
								BadValue: r.Spec.TargetVersion,
								Detail:   "CustomContainerImages must be updated when changing targetVersion. The current CustomContainerImages are identical to those used in the previous version (" + oldVersion.Spec.TargetVersion + "), which prevents proper version tracking and validation.",
							},
						)
					}
				}
			}
		}
	}

	return nil, nil
}

// hasAnyCustomImage checks if any image field in CustomContainerImages is set
func hasAnyCustomImage(images CustomContainerImages) bool {
	// Check CinderVolumeImages map
	for _, img := range images.CinderVolumeImages {
		if img != nil {
			return true
		}
	}

	// Check ManilaShareImages map
	for _, img := range images.ManilaShareImages {
		if img != nil {
			return true
		}
	}

	// Check ContainerTemplate fields using reflection
	v := reflect.ValueOf(images.ContainerTemplate)
	t := reflect.TypeOf(images.ContainerTemplate)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Check if field is a pointer and not nil
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			// Additional check to ensure it's a string pointer (image fields)
			if fieldType.Type.Elem().Kind() == reflect.String {
				return true
			}
		}
	}

	return false
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
