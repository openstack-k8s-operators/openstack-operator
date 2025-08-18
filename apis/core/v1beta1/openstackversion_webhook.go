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
		// When changing target version during a minor update, ensure CustomContainerImages are also updated
		if len(r.Spec.CustomContainerImages.CinderVolumeImages) > 0 ||
			len(r.Spec.CustomContainerImages.ManilaShareImages) > 0 ||
			hasAnyCustomImage(r.Spec.CustomContainerImages.ContainerTemplate) {

			// Get the tracked custom images for the previous version
			if r.Status.TrackedCustomImages != nil {
				if trackedImages, exists := r.Status.TrackedCustomImages[oldVersion.Spec.TargetVersion]; exists {
					// Compare current CustomContainerImages with tracked ones
					if customContainerImagesEqual(r.Spec.CustomContainerImages, trackedImages) {
						return nil, apierrors.NewForbidden(
							schema.GroupResource{
								Group:    GroupVersion.WithKind("OpenStackVersion").Group,
								Resource: GroupVersion.WithKind("OpenStackVersion").Kind,
							}, r.GetName(), &field.Error{
								Type:     field.ErrorTypeForbidden,
								Field:    "spec.customContainerImages",
								BadValue: r.Spec.TargetVersion,
								Detail:   "CustomContainerImages must be updated when changing targetVersion during a minor update. The current CustomContainerImages are identical to those used in the previous version (" + oldVersion.Spec.TargetVersion + "), which prevents proper version tracking and validation.",
							},
						)
					}
				}
			}
		}
	}

	return nil, nil
}

// hasAnyCustomImage checks if any image field in ContainerTemplate is set
func hasAnyCustomImage(template ContainerTemplate) bool {
	return template.AgentImage != nil ||
		template.AnsibleeeImage != nil ||
		template.AodhAPIImage != nil ||
		template.AodhEvaluatorImage != nil ||
		template.AodhListenerImage != nil ||
		template.AodhNotifierImage != nil ||
		template.ApacheImage != nil ||
		template.BarbicanAPIImage != nil ||
		template.BarbicanKeystoneListenerImage != nil ||
		template.BarbicanWorkerImage != nil ||
		template.CeilometerCentralImage != nil ||
		template.CeilometerComputeImage != nil ||
		template.CeilometerIpmiImage != nil ||
		template.CeilometerNotificationImage != nil ||
		template.CeilometerSgcoreImage != nil ||
		template.CeilometerMysqldExporterImage != nil ||
		template.CinderAPIImage != nil ||
		template.CinderBackupImage != nil ||
		template.CinderSchedulerImage != nil ||
		template.DesignateAPIImage != nil ||
		template.DesignateBackendbind9Image != nil ||
		template.DesignateCentralImage != nil ||
		template.DesignateMdnsImage != nil ||
		template.DesignateProducerImage != nil ||
		template.DesignateUnboundImage != nil ||
		template.DesignateWorkerImage != nil ||
		template.EdpmFrrImage != nil ||
		template.EdpmIscsidImage != nil ||
		template.EdpmLogrotateCrondImage != nil ||
		template.EdpmMultipathdImage != nil ||
		template.EdpmNeutronDhcpAgentImage != nil ||
		template.EdpmNeutronMetadataAgentImage != nil ||
		template.EdpmNeutronOvnAgentImage != nil ||
		template.EdpmNeutronSriovAgentImage != nil ||
		template.EdpmOvnBgpAgentImage != nil ||
		template.EdpmNodeExporterImage != nil ||
		template.EdpmKeplerImage != nil ||
		template.EdpmPodmanExporterImage != nil ||
		template.EdpmOpenstackNetworkExporterImage != nil ||
		template.OpenstackNetworkExporterImage != nil ||
		template.GlanceAPIImage != nil ||
		template.HeatAPIImage != nil ||
		template.HeatCfnapiImage != nil ||
		template.HeatEngineImage != nil ||
		template.HorizonImage != nil ||
		template.InfraDnsmasqImage != nil ||
		template.InfraMemcachedImage != nil ||
		template.InfraRedisImage != nil ||
		template.IronicAPIImage != nil ||
		template.IronicConductorImage != nil ||
		template.IronicInspectorImage != nil ||
		template.IronicNeutronAgentImage != nil ||
		template.IronicPxeImage != nil ||
		template.IronicPythonAgentImage != nil ||
		template.KeystoneAPIImage != nil ||
		template.KsmImage != nil ||
		template.ManilaAPIImage != nil ||
		template.ManilaSchedulerImage != nil ||
		template.MariadbImage != nil ||
		template.NetUtilsImage != nil ||
		template.NeutronAPIImage != nil ||
		template.NovaAPIImage != nil ||
		template.NovaComputeImage != nil ||
		template.NovaConductorImage != nil ||
		template.NovaNovncImage != nil ||
		template.NovaSchedulerImage != nil ||
		template.OctaviaAPIImage != nil ||
		template.OctaviaHealthmanagerImage != nil ||
		template.OctaviaHousekeepingImage != nil ||
		template.OctaviaWorkerImage != nil ||
		template.OctaviaRsyslogImage != nil ||
		template.OpenstackClientImage != nil ||
		template.OsContainerImage != nil ||
		template.OvnControllerImage != nil ||
		template.OvnControllerOvsImage != nil ||
		template.OvnNbDbclusterImage != nil ||
		template.OvnNorthdImage != nil ||
		template.OvnSbDbclusterImage != nil ||
		template.PlacementAPIImage != nil ||
		template.RabbitmqImage != nil ||
		template.SwiftAccountImage != nil ||
		template.SwiftContainerImage != nil ||
		template.SwiftObjectImage != nil ||
		template.SwiftProxyImage != nil ||
		template.TelemetryNodeExporterImage != nil ||
		template.TestTempestImage != nil ||
		template.TestTobikoImage != nil ||
		template.TestHorizontestImage != nil ||
		template.TestAnsibletestImage != nil ||
		template.WatcherAPIImage != nil ||
		template.WatcherApplierImage != nil ||
		template.WatcherDecisionEngineImage != nil
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
