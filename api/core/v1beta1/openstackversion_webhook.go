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
	"fmt"
	"os"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"
	goClient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

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

// Default sets default values for the OpenStackVersion
func (r *OpenStackVersion) Default() {
	openstackversionlog.Info("default", "name", r.Name)
	if r.Spec.TargetVersion == "" {
		r.Spec.TargetVersion = openstackVersionDefaults.AvailableVersion
	}
}

// ValidateCreate validates the OpenStackVersion on creation
func (r *OpenStackVersion) ValidateCreate(ctx context.Context, c goClient.Client) (admission.Warnings, error) {
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

	if err := validateMinorUpdateTargetStageAnnotation(r.Annotations, r.GetName()); err != nil {
		return nil, err
	}

	versionList, err := GetOpenStackVersions(r.Namespace, c)

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

// ValidateUpdate validates the OpenStackVersion on update
func (r *OpenStackVersion) ValidateUpdate(ctx context.Context, old runtime.Object, c goClient.Client) (admission.Warnings, error) {
	openstackversionlog.Info("validate update", "name", r.Name)

	if err := validateMinorUpdateTargetStageAnnotation(r.Annotations, r.GetName()); err != nil {
		return nil, err
	}

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

	// Validate that the target stage annotation is not from earlier stage while a minor update is in progress
	if err := validateMinorUpdateTargetStageAnnotationProgress(oldVersion, r); err != nil {
		return nil, err
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

func validateMinorUpdateTargetStageAnnotation(annotations map[string]string, resourceName string) error {
	if annotations == nil {
		return nil
	}
	stage, ok := annotations[MinorUpdateTargetStageAnnotation]
	if !ok {
		return nil
	}
	annotationField := "metadata.annotations[" + MinorUpdateTargetStageAnnotation + "]"
	if stage == "" {
		return apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, resourceName, &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    annotationField,
				BadValue: stage,
				Detail:   "Annotation value must not be empty. Remove the annotation or set a valid stage name",
			},
		)
	}
	if !IsValidMinorUpdateTargetStage(stage) {
		return apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackVersion").Group,
				Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
			}, resourceName, &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    annotationField,
				BadValue: stage,
				Detail: fmt.Sprintf(
					"Invalid target stage %q. Must be one of: %s",
					stage,
					strings.Join(ValidMinorUpdateTargetStages(), ", "),
				),
			},
		)
	}
	return nil
}

func minorUpdateInProgress(v *OpenStackVersion) bool {
	if v.Status.DeployedVersion == nil {
		return false
	}
	return v.Spec.TargetVersion != *v.Status.DeployedVersion
}

// validateMinorUpdateTargetStageAnnotationProgress rejects moving the target-stage
// annotation to an earlier rollout stage while a minor update is in progress, and rejects
// adding the annotation behind stages already completed when it was absent at update start.
func validateMinorUpdateTargetStageAnnotationProgress(old, new *OpenStackVersion) error {
	if !minorUpdateInProgress(new) {
		return nil
	}
	oldStage, oldOK := MinorUpdateTargetStageFromAnnotations(old.Annotations)
	newStage, newOK := MinorUpdateTargetStageFromAnnotations(new.Annotations)
	if !newOK {
		return nil
	}
	newIdx, okNew := MinorUpdateTargetStageIndex(newStage)
	if !okNew {
		return nil
	}
	annotationField := "metadata.annotations[" + MinorUpdateTargetStageAnnotation + "]"
	gr := schema.GroupResource{
		Group:    GroupVersion.WithKind("OpenStackVersion").Group,
		Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
	}

	if !oldOK {
		latest := LatestCompletedMinorUpdateTargetStageIndex(old.Status)
		if latest >= 0 && newIdx < latest {
			completedStage := validMinorUpdateTargetStagesOrdered[latest]
			return apierrors.NewForbidden(
				gr, new.GetName(), &field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    annotationField,
					BadValue: newStage,
					Detail: fmt.Sprintf(
						"Cannot set update target stage to %q while minor update is in progress: update has already completed stage %q (targetVersion %q, deployedVersion %q); choose a further stage",
						newStage, completedStage, new.Spec.TargetVersion, *new.Status.DeployedVersion,
					),
				},
			)
		}
		return nil
	}

	oldIdx, _ := MinorUpdateTargetStageIndex(oldStage)
	if newIdx >= oldIdx {
		return nil
	}
	return apierrors.NewForbidden(
		gr, new.GetName(), &field.Error{
			Type:     field.ErrorTypeForbidden,
			Field:    annotationField,
			BadValue: newStage,
			Detail: fmt.Sprintf(
				"Cannot move update target stage from %q to earlier stage %q while minor update is in progress (targetVersion %q, deployedVersion %q); remove the annotation or set a further stage",
				oldStage, newStage, new.Spec.TargetVersion, *new.Status.DeployedVersion,
			),
		},
	)
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

// ValidateDelete validates the OpenStackVersion on deletion
func (r *OpenStackVersion) ValidateDelete(ctx context.Context, c goClient.Client) (admission.Warnings, error) {
	openstackversionlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

// SetupVersionDefaults -
func SetupVersionDefaults() {
	openstackVersionDefaults := OpenStackVersionDefaults{
		AvailableVersion: GetOpenStackReleaseVersion(os.Environ()),
	}

	SetupOpenStackVersionDefaults(openstackVersionDefaults)
}
