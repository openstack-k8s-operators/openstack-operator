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
	"strings"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/route"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
)

var ctlplaneWebhookClient client.Client

// OpenStackControlPlaneDefaults -
type OpenStackControlPlaneDefaults struct {
	RabbitMqImageURL string
}

var openstackControlPlaneDefaults OpenStackControlPlaneDefaults

// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

// SetupOpenStackControlPlaneDefaults - initialize OpenStackControlPlane spec defaults for use with internal webhooks
func SetupOpenStackControlPlaneDefaults(defaults OpenStackControlPlaneDefaults) {
	openstackControlPlaneDefaults = defaults
	openstackcontrolplanelog.Info("OpenStackControlPlane defaults initialized", "defaults", defaults)
}

// SetupWebhookWithManager sets up the Webhook with the Manager.
func (r *OpenStackControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if ctlplaneWebhookClient == nil {
		ctlplaneWebhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=Fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() (admission.Warnings, error) {
	openstackcontrolplanelog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	var allWarn []string
	basePath := field.NewPath("spec")

	ctlplaneList := &OpenStackControlPlaneList{}
	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}
	if err := ctlplaneWebhookClient.List(context.TODO(), ctlplaneList, listOpts...); err != nil {
		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackControlPlane").Group,
				Resource: GroupVersion.WithKind("OpenStackControlPlane").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "",
				BadValue: r.Name,
				Detail:   err.Error(),
			},
		)
	}
	if len(ctlplaneList.Items) >= 1 {
		return nil, apierrors.NewForbidden(
			schema.GroupResource{
				Group:    GroupVersion.WithKind("OpenStackControlPlane").Group,
				Resource: GroupVersion.WithKind("OpenStackControlPlane").Kind,
			}, r.GetName(), &field.Error{
				Type:     field.ErrorTypeForbidden,
				Field:    "",
				BadValue: r.Name,
				Detail:   "Only one OpenStackControlPlane instance per namespace is supported at this time.",
			},
		)
	}

	allWarn, allErrs = r.ValidateCreateServices(basePath)
	if len(allErrs) != 0 {
		return allWarn, apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return allWarn, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	openstackcontrolplanelog.Info("validate update", "name", r.Name)

	oldControlPlane, ok := old.(*OpenStackControlPlane)
	if !ok || oldControlPlane == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.ValidateUpdateServices(oldControlPlane.Spec, basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() (admission.Warnings, error) {
	openstackcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil, nil
}

// checkDepsEnabled - returns a non-empty string if required services are missing (disabled) for "name" service
func (r *OpenStackControlPlane) checkDepsEnabled(name string) string {
	// "msg" will hold any dependency validation error we might find
	msg := ""
	// "reqs" will be set to the required services for "name" service
	// if any of those required services are improperly disabled/missing
	reqs := ""

	switch name {
	case "Keystone":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ"
		}
	case "Glance":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Galera, Memcached, Keystone"
		}
	case "Cinder":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ, Keystone"
		}
	case "Placement":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Galera, Memcached, Keystone"
		}
	case "Neutron":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "Galera, RabbitMQ, Keystone"
		}
	case "Nova":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Placement.Enabled && r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ, Keystone, Glance Neutron, Placement"
		}
	case "Heat":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ, Keystone"
		}
	case "Swift":
		if !(r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Memcached, Keystone"
		}
	case "Horizon":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Galera, Memcached, Keystone"
		}
	case "Barbican":
		if !((r.Spec.Galera.Enabled) && r.Spec.Keystone.Enabled) {
			reqs = "Galera, Keystone"
		}
	case "Octavia":
		if !((r.Spec.Galera.Enabled) && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled && r.Spec.Nova.Enabled &&
			r.Spec.Ovn.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ, Keystone, Glance, Neutron, Nova, OVN"
		}
	case "Telemetry.Autoscaling":
		if !(r.Spec.Galera.Enabled && r.Spec.Heat.Enabled && r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "Galera, Heat, RabbitMQ, Keystone"
		}
	case "Telemetry.Ceilometer":
		if !(r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "RabbitMQ, Keystone"
		}
	}

	// If "reqs" is not the empty string, we have missing requirements
	if reqs != "" {
		msg = fmt.Sprintf("%s requires these services to be enabled: %s.", name, reqs)
	}

	return msg
}

// ValidateCreateServices validating service definitions during the OpenstackControlPlane CR creation
func (r *OpenStackControlPlane) ValidateCreateServices(basePath *field.Path) (admission.Warnings, field.ErrorList) {
	var errors field.ErrorList
	var warnings []string

	errors = append(errors, r.ValidateServiceDependencies(basePath)...)

	// Call internal validation logic for individual service operators
	if r.Spec.Keystone.Enabled {
		errors = append(errors, r.Spec.Keystone.Template.ValidateCreate(basePath.Child("keystone").Child("template"))...)
	}

	if r.Spec.Ironic.Enabled {
		errors = append(errors, r.Spec.Ironic.Template.ValidateCreate(basePath.Child("ironic").Child("template"))...)
	}

	if r.Spec.Nova.Enabled {
		errors = append(errors, r.Spec.Nova.Template.ValidateCreate(basePath.Child("nova").Child("template"))...)
	}

	if r.Spec.Placement.Enabled {
		errors = append(errors, r.Spec.Placement.Template.ValidateCreate(basePath.Child("placement").Child("template"))...)
	}

	if r.Spec.Barbican.Enabled {
		errors = append(errors, r.Spec.Barbican.Template.ValidateCreate(basePath.Child("barbican").Child("template"))...)
	}

	if r.Spec.Neutron.Enabled {
		errors = append(errors, r.Spec.Neutron.Template.ValidateCreate(basePath.Child("neutron").Child("template"))...)
	}

	if r.Spec.Glance.Enabled {
		errors = append(errors, r.Spec.Glance.Template.ValidateCreate(basePath.Child("glance").Child("template"))...)
	}

	if r.Spec.Cinder.Enabled {
		errors = append(errors, r.Spec.Cinder.Template.ValidateCreate(basePath.Child("cinder").Child("template"))...)
	}

	if r.Spec.Heat.Enabled {
		errors = append(errors, r.Spec.Heat.Template.ValidateCreate(basePath.Child("heat").Child("template"))...)
	}

	if r.Spec.Manila.Enabled {
		errors = append(errors, r.Spec.Manila.Template.ValidateCreate(basePath.Child("manila").Child("template"))...)
	}

	if r.Spec.Swift.Enabled {
		errors = append(errors, r.Spec.Swift.Template.ValidateCreate(basePath.Child("swift").Child("template"))...)
	}

	if r.Spec.Octavia.Enabled {
		errors = append(errors, r.Spec.Octavia.Template.ValidateCreate(basePath.Child("octavia").Child("template"))...)
	}

	if r.Spec.Designate.Enabled {
		errors = append(errors, r.Spec.Designate.Template.ValidateCreate(basePath.Child("designate").Child("template"))...)
	}

	if r.Spec.Galera.Enabled {
		for key, s := range *r.Spec.Galera.Templates {
			warn, err := s.ValidateCreate(basePath.Child("galera").Child("template").Key(key))
			errors = append(errors, err...)
			warnings = append(warnings, warn...)
		}
	}

	return warnings, errors
}

// ValidateUpdateServices validating service definitions during the OpenstackControlPlane CR update
func (r *OpenStackControlPlane) ValidateUpdateServices(old OpenStackControlPlaneSpec, basePath *field.Path) field.ErrorList {
	var errors field.ErrorList

	errors = append(errors, r.ValidateServiceDependencies(basePath)...)

	// Call internal validation logic for individual service operators
	if r.Spec.Keystone.Enabled {
		if old.Keystone.Template == nil {
			old.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
		}
		errors = append(errors, r.Spec.Keystone.Template.ValidateUpdate(*old.Keystone.Template, basePath.Child("keystone").Child("template"))...)
	}

	if r.Spec.Ironic.Enabled {
		if old.Ironic.Template == nil {
			old.Ironic.Template = &ironicv1.IronicSpecCore{}
		}
		errors = append(errors, r.Spec.Ironic.Template.ValidateUpdate(*old.Ironic.Template, basePath.Child("ironic").Child("template"))...)
	}

	if r.Spec.Nova.Enabled {
		if old.Nova.Template == nil {
			old.Nova.Template = &novav1.NovaSpec{}
		}
		errors = append(errors, r.Spec.Nova.Template.ValidateUpdate(*old.Nova.Template, basePath.Child("nova").Child("template"))...)
	}

	if r.Spec.Placement.Enabled {
		if old.Placement.Template == nil {
			old.Placement.Template = &placementv1.PlacementAPISpecCore{}
		}
		errors = append(errors, r.Spec.Placement.Template.ValidateUpdate(*old.Placement.Template, basePath.Child("placement").Child("template"))...)
	}

	if r.Spec.Barbican.Enabled {
		if old.Barbican.Template == nil {
			old.Barbican.Template = &barbicanv1.BarbicanSpecCore{}
		}
		errors = append(errors, r.Spec.Barbican.Template.ValidateUpdate(*old.Barbican.Template, basePath.Child("barbican").Child("template"))...)
	}

	if r.Spec.Neutron.Enabled {
		if old.Neutron.Template == nil {
			old.Neutron.Template = &neutronv1.NeutronAPISpecCore{}
		}
		errors = append(errors, r.Spec.Neutron.Template.ValidateUpdate(*old.Neutron.Template, basePath.Child("neutron").Child("template"))...)
	}

	if r.Spec.Glance.Enabled {
		if old.Glance.Template == nil {
			old.Glance.Template = &glancev1.GlanceSpecCore{}
		}
		errors = append(errors, r.Spec.Glance.Template.ValidateUpdate(*old.Glance.Template, basePath.Child("glance").Child("template"))...)
	}

	if r.Spec.Cinder.Enabled {
		if old.Cinder.Template == nil {
			old.Cinder.Template = &cinderv1.CinderSpecCore{}
		}
		errors = append(errors, r.Spec.Cinder.Template.ValidateUpdate(*old.Cinder.Template, basePath.Child("cinder").Child("template"))...)
	}

	if r.Spec.Heat.Enabled {
		if old.Heat.Template == nil {
			old.Heat.Template = &heatv1.HeatSpecCore{}
		}
		errors = append(errors, r.Spec.Heat.Template.ValidateUpdate(*old.Heat.Template, basePath.Child("heat").Child("template"))...)
	}

	if r.Spec.Manila.Enabled {
		if old.Manila.Template == nil {
			old.Manila.Template = &manilav1.ManilaSpecCore{}
		}
		errors = append(errors, r.Spec.Manila.Template.ValidateUpdate(*old.Manila.Template, basePath.Child("manila").Child("template"))...)
	}

	if r.Spec.Swift.Enabled {
		if old.Swift.Template == nil {
			old.Swift.Template = &swiftv1.SwiftSpecCore{}
		}
		errors = append(errors, r.Spec.Swift.Template.ValidateUpdate(*old.Swift.Template, basePath.Child("swift").Child("template"))...)
	}

	if r.Spec.Octavia.Enabled {
		if old.Octavia.Template == nil {
			old.Octavia.Template = &octaviav1.OctaviaSpecCore{}
		}
		errors = append(errors, r.Spec.Octavia.Template.ValidateUpdate(*old.Octavia.Template, basePath.Child("octavia").Child("template"))...)
	}

	if r.Spec.Designate.Enabled {
		if old.Designate.Template == nil {
			old.Designate.Template = &designatev1.DesignateSpecCore{}
		}
		errors = append(errors, r.Spec.Designate.Template.ValidateUpdate(*old.Designate.Template, basePath.Child("designate").Child("template"))...)
	}

	return errors
}

// ValidateServiceDependencies ensures that when a service is enabled then all the services it depends on are also
// enabled
func (r *OpenStackControlPlane) ValidateServiceDependencies(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Add service dependency validations

	if r.Spec.Keystone.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Keystone"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("keystone").Child("enabled"), r.Spec.Keystone.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Glance.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Glance"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("glance").Child("enabled"), r.Spec.Glance.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Cinder.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Cinder"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("cinder").Child("enabled"), r.Spec.Cinder.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Placement.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Placement"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("placement").Child("enabled"), r.Spec.Placement.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Neutron.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Neutron"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("neutron").Child("enabled"), r.Spec.Neutron.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Nova.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Nova"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("nova").Child("enabled"), r.Spec.Nova.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Heat.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Heat"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("heat").Child("enabled"), r.Spec.Heat.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Swift.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Swift"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("swift").Child("enabled"), r.Spec.Swift.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Horizon.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Horizon"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("horizon").Child("enabled"), r.Spec.Horizon.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Octavia.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Octavia"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("octavia").Child("enabled"), r.Spec.Octavia.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.Barbican.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Barbican"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("barbican").Child("enabled"), r.Spec.Barbican.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}
	if r.Spec.Telemetry.Enabled &&
		r.Spec.Telemetry.Template.Ceilometer.Enabled != nil &&
		*r.Spec.Telemetry.Template.Ceilometer.Enabled {

		if depErrorMsg := r.checkDepsEnabled("Telemetry.Ceilometer"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("telemetry").Child("template").Child("ceilometer").Child("enabled"),
				*r.Spec.Telemetry.Template.Ceilometer.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}
	if r.Spec.Telemetry.Enabled &&
		r.Spec.Telemetry.Template.Autoscaling.Enabled != nil &&
		*r.Spec.Telemetry.Template.Autoscaling.Enabled {

		if depErrorMsg := r.checkDepsEnabled("Telemetry.Autoscaling"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("telemetry").Child("template").Child("autoscaling").Child("enabled"),
				*r.Spec.Telemetry.Template.Autoscaling.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}

//+kubebuilder:webhook:path=/mutate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=mopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackControlPlane) Default() {
	openstackcontrolplanelog.Info("default", "name", r.Name)

	r.DefaultLabel()
	r.DefaultServices()
}

// Helper function to initialize overrideSpec object. Could be moved to lib-common.
func initializeOverrideSpec(override **route.OverrideSpec, initAnnotations bool) {
	if *override == nil {
		*override = &route.OverrideSpec{}
	}
	if initAnnotations {
		if (*override).EmbeddedLabelsAnnotations == nil {
			(*override).EmbeddedLabelsAnnotations = &route.EmbeddedLabelsAnnotations{}
		}
		if (*override).Annotations == nil {
			(*override).Annotations = make(map[string]string)
		}
	}
}

func setOverrideSpec(override **route.OverrideSpec, anno map[string]string) {
	initializeOverrideSpec(override, false)
	(*override).AddAnnotation(anno)
}

// DefaultServices - common function for calling individual services' defaulting functions
func (r *OpenStackControlPlane) DefaultServices() {
	// Cinder
	if r.Spec.Cinder.Enabled || r.Spec.Cinder.Template != nil {
		if r.Spec.Cinder.Template == nil {
			r.Spec.Cinder.Template = &cinderv1.CinderSpecCore{}
		}
		r.Spec.Cinder.Template.Default()
		initializeOverrideSpec(&r.Spec.Cinder.APIOverride.Route, true)
		r.Spec.Cinder.Template.SetDefaultRouteAnnotations(r.Spec.Cinder.APIOverride.Route.Annotations)
	}

	// Galera
	if r.Spec.Galera.Enabled || r.Spec.Galera.Templates != nil {
		if r.Spec.Galera.Templates == nil {
			r.Spec.Galera.Templates = ptr.To(map[string]mariadbv1.GaleraSpecCore{})
		}

		for key, template := range *r.Spec.Galera.Templates {
			if template.StorageClass == "" {
				template.StorageClass = r.Spec.StorageClass
			}
			if template.Secret == "" {
				template.Secret = r.Spec.Secret
			}
			template.Default()
			// By-value copy, need to update
			(*r.Spec.Galera.Templates)[key] = template
		}
	}

	// Glance
	if r.Spec.Glance.Enabled || r.Spec.Glance.Template != nil {
		if r.Spec.Glance.Template == nil {
			r.Spec.Glance.Template = &glancev1.GlanceSpecCore{}
		}
		r.Spec.Glance.Template.Default()
		// initialize the main APIOverride struct
		if r.Spec.Glance.APIOverride == nil {
			r.Spec.Glance.APIOverride = map[string]Override{}
		}
		for name, glanceAPI := range r.Spec.Glance.Template.GlanceAPIs {
			var override Override
			var ok bool

			if override, ok = r.Spec.Glance.APIOverride[name]; !ok {
				override = Override{}
			}
			// Do not build APIOverrides for an APIEdge instance
			if glanceAPI.Type != glancev1.APIEdge {
				initializeOverrideSpec(&override.Route, true)
				glanceAPI.SetDefaultRouteAnnotations(override.Route.Annotations)
				r.Spec.Glance.APIOverride[name] = override
			}
		}
		// clean up the APIOverrides for each glanceAPI that has been
		// deleted from the ctlplane
		apis := maps.Keys(r.Spec.Glance.Template.GlanceAPIs)
		for k, _ := range r.Spec.Glance.APIOverride {
			if !slices.Contains(apis, k) {
				delete(r.Spec.Glance.APIOverride, k)
			}
		}
	}

	// Ironic
	if r.Spec.Ironic.Enabled || r.Spec.Ironic.Template != nil {
		if r.Spec.Ironic.Template == nil {
			r.Spec.Ironic.Template = &ironicv1.IronicSpecCore{}
		}

		// Default Secret
		if r.Spec.Ironic.Template.Secret == "" {
			r.Spec.Ironic.Template.Secret = r.Spec.Secret
		}
		// Default DatabaseInstance
		if r.Spec.Ironic.Template.DatabaseInstance == "" {
			r.Spec.Ironic.Template.DatabaseInstance = "openstack"
		}
		// Default StorageClass
		if r.Spec.Ironic.Template.StorageClass == "" {
			r.Spec.Ironic.Template.StorageClass = r.Spec.StorageClass
		}
		r.Spec.Ironic.Template.Default()
	}

	// Keystone
	if r.Spec.Keystone.Enabled || r.Spec.Keystone.Template != nil {
		if r.Spec.Keystone.Template == nil {
			r.Spec.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
		}
		r.Spec.Keystone.Template.Default()
	}

	// Manila
	if r.Spec.Manila.Enabled || r.Spec.Manila.Template != nil {
		if r.Spec.Manila.Template == nil {
			r.Spec.Manila.Template = &manilav1.ManilaSpecCore{}
		}
		r.Spec.Manila.Template.Default()
		initializeOverrideSpec(&r.Spec.Manila.APIOverride.Route, true)
		r.Spec.Manila.Template.SetDefaultRouteAnnotations(r.Spec.Manila.APIOverride.Route.Annotations)
	}

	// Memcached
	if r.Spec.Memcached.Enabled || r.Spec.Memcached.Templates != nil {
		if r.Spec.Memcached.Templates == nil {
			r.Spec.Memcached.Templates = ptr.To(map[string]memcachedv1.MemcachedSpecCore{})
		}

		for key, template := range *r.Spec.Memcached.Templates {
			template.Default()
			// By-value copy, need to update
			(*r.Spec.Memcached.Templates)[key] = template
		}
	}

	// Neutron
	if r.Spec.Neutron.Enabled || r.Spec.Neutron.Template != nil {
		if r.Spec.Neutron.Template == nil {
			r.Spec.Neutron.Template = &neutronv1.NeutronAPISpecCore{}
		}
		r.Spec.Neutron.Template.Default()
		setOverrideSpec(&r.Spec.Neutron.APIOverride.Route, r.Spec.Neutron.Template.GetDefaultRouteAnnotations())
	}

	// Nova
	if r.Spec.Nova.Enabled || r.Spec.Nova.Template != nil {
		if r.Spec.Nova.Template == nil {
			r.Spec.Nova.Template = &novav1.NovaSpec{}
		}
		r.Spec.Nova.Template.Default()
	}

	// OVN
	if r.Spec.Ovn.Enabled || r.Spec.Ovn.Template != nil {
		if r.Spec.Ovn.Template == nil {
			r.Spec.Ovn.Template = &OvnResources{}
		}

		for key, template := range r.Spec.Ovn.Template.OVNDBCluster {
			template.Default()
			// By-value copy, need to update
			r.Spec.Ovn.Template.OVNDBCluster[key] = template
		}

		r.Spec.Ovn.Template.OVNNorthd.Default()
		r.Spec.Ovn.Template.OVNController.Default()
	}

	// Placement
	if r.Spec.Placement.Enabled || r.Spec.Placement.Template != nil {
		if r.Spec.Placement.Template == nil {
			r.Spec.Placement.Template = &placementv1.PlacementAPISpecCore{}
		}
		r.Spec.Placement.Template.Default()
	}

	// DNS
	if r.Spec.DNS.Enabled || r.Spec.DNS.Template != nil {
		if r.Spec.DNS.Template == nil {
			r.Spec.DNS.Template = &networkv1.DNSMasqSpec{}
		}

		r.Spec.DNS.Template.Default()
	}

	// Telemetry
	if r.Spec.Telemetry.Enabled || r.Spec.Telemetry.Template != nil {
		if r.Spec.Telemetry.Template == nil {
			r.Spec.Telemetry.Template = &telemetryv1.TelemetrySpecCore{}
		}
		r.Spec.Telemetry.Template.Default()
	}

	// Heat
	if r.Spec.Heat.Enabled || r.Spec.Heat.Template != nil {
		if r.Spec.Heat.Template == nil {
			r.Spec.Heat.Template = &heatv1.HeatSpecCore{}
		}
		r.Spec.Heat.Template.Default()
	}

	// Swift
	if r.Spec.Swift.Enabled || r.Spec.Swift.Template != nil {
		if r.Spec.Swift.Template == nil {
			r.Spec.Swift.Template = &swiftv1.SwiftSpecCore{}
		}

		if r.Spec.Swift.Template.SwiftStorage.StorageClass == "" {
			r.Spec.Swift.Template.SwiftStorage.StorageClass = r.Spec.StorageClass
		}

		r.Spec.Swift.Template.Default()
	}

	// Horizon
	if r.Spec.Horizon.Enabled || r.Spec.Horizon.Template != nil {
		if r.Spec.Horizon.Template == nil {
			r.Spec.Horizon.Template = &horizonv1.HorizonSpecCore{}
		}

		r.Spec.Horizon.Template.Default()
	}

	// Octavia
	if r.Spec.Octavia.Enabled || r.Spec.Octavia.Template != nil {
		if r.Spec.Octavia.Template == nil {
			r.Spec.Octavia.Template = &octaviav1.OctaviaSpecCore{}
		}

		r.Spec.Octavia.Template.Default()
	}

	// Barbican
	if r.Spec.Barbican.Enabled || r.Spec.Barbican.Template != nil {
		if r.Spec.Barbican.Template == nil {
			r.Spec.Barbican.Template = &barbicanv1.BarbicanSpecCore{}
		}
		r.Spec.Barbican.Template.Default()
	}

	// Designate
	if r.Spec.Designate.Enabled || r.Spec.Designate.Template != nil {
		if r.Spec.Designate.Template == nil {
			r.Spec.Designate.Template = &designatev1.DesignateSpecCore{}
		}
		r.Spec.Designate.Template.Default()
	}
}

// DefaultLabel - adding default label to the OpenStackControlPlane
func (r *OpenStackControlPlane) DefaultLabel() {
	// adds map[string]string{"core.openstack.org/openstackcontrolplane": r.name>} to the
	// instance, if not already provided in the CR. With this ctlplane object can be
	// queried using the default label.
	typeLabel := strings.ToLower(r.GroupVersionKind().Group + "/" + r.Kind)
	if _, ok := r.Labels[typeLabel]; !ok {
		if r.Labels == nil {
			r.Labels = map[string]string{}
		}
		r.Labels[typeLabel] = ""
	}
}
