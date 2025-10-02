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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/object"
	"github.com/openstack-k8s-operators/lib-common/modules/common/route"
	common_webhook "github.com/openstack-k8s-operators/lib-common/modules/common/webhook"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	watcherv1 "github.com/openstack-k8s-operators/watcher-operator/api/v1beta1"
	"golang.org/x/exp/maps"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
)

// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

// generateRandomID generates a random 5-character hexadecimal ID
// Used for service naming when UniquePodNames is enabled and UID is not yet available
func generateRandomID() (string, error) {
	bytes := make([]byte, 3) // 3 bytes = 6 hex chars, we'll take first 5
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:5], nil
}

// lookupServiceCR attempts to find an existing service CR in the cluster owned by this OpenStackControlPlane
// Returns the CR name if found, empty string if not found or not owned by the given owner UID
// serviceName should be the base service name (e.g., CinderName, GlanceName)
// ownerUID is the UID of the OpenStackControlPlane that should own the CR
// This function lists CRs and finds ones that start with the service name prefix and are owned by ownerUID
func lookupServiceCR(ctx context.Context, c client.Client, namespace, serviceName string, ownerUID types.UID) (string, error) {
	switch serviceName {
	case CinderName:
		cinderList := &cinderv1.CinderList{}
		if err := c.List(ctx, cinderList, client.InNamespace(namespace)); err != nil {
			return "", fmt.Errorf("failed to list Cinder CRs: %w", err)
		}
		// Find any Cinder CR that starts with "cinder" and is owned by this OpenStackControlPlane
		for _, cinder := range cinderList.Items {
			if strings.HasPrefix(cinder.Name, CinderName) && object.CheckOwnerRefExist(ownerUID, cinder.GetOwnerReferences()) {
				return cinder.Name, nil
			}
		}

	case GlanceName:
		glanceList := &glancev1.GlanceList{}
		if err := c.List(ctx, glanceList, client.InNamespace(namespace)); err != nil {
			return "", fmt.Errorf("failed to list Glance CRs: %w", err)
		}
		// Find any Glance CR that starts with "glance" and is owned by this OpenStackControlPlane
		for _, glance := range glanceList.Items {
			if strings.HasPrefix(glance.Name, GlanceName) && object.CheckOwnerRefExist(ownerUID, glance.GetOwnerReferences()) {
				return glance.Name, nil
			}
		}

	default:
		return "", fmt.Errorf("unsupported service name: %s", serviceName)
	}

	return "", nil // Not found or not owned
}

// CacheServiceNameForCreate handles service name caching during CREATE operations
// Generates a random ID since UID is not yet available
func (r *OpenStackControlPlane) CacheServiceNameForCreate(serviceName string) (string, error) {
	randomID, err := generateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return fmt.Sprintf("%s-%s", serviceName, randomID), nil
}

// CacheServiceNameForUpdate handles service name caching during UPDATE operations
// Uses existing CR name if it's owned by this OpenStackControlPlane, otherwise generates based on current settings
// This provides robust flip detection: if we created a CR previously, we preserve its name to avoid creating duplicates
func (r *OpenStackControlPlane) CacheServiceNameForUpdate(ctx context.Context, c client.Client, serviceName string) (string, error) {
	// Lookup existing CR owned by this OpenStackControlPlane
	existingName, err := lookupServiceCR(ctx, c, r.Namespace, serviceName, r.UID)
	if err != nil {
		return "", fmt.Errorf("failed to lookup existing CR: %w", err)
	}

	// If we find a CR owned by us, preserve its name regardless of format
	// This handles both flip scenarios and prevents creating duplicate CRs:
	// - If UniquePodNames changed from false→true, we keep the old "cinder" name
	// - If UniquePodNames changed from true→false, we keep the old "cinder-abc" name
	// - If UniquePodNames didn't change, we keep the existing name
	if existingName != "" {
		return existingName, nil
	}

	// No existing CR found owned by us - generate name based on current UniquePodNames setting
	// This handles:
	// - First time deployment
	// - Operator upgrade scenarios where ServiceName wasn't cached yet
	name, _ := r.GetServiceName(serviceName, true)
	if name == serviceName {
		// GetServiceName returned base name, meaning UID is not available
		return "", fmt.Errorf("unable to generate service name: no existing CR and UID not available")
	}
	return name, nil
}

// ValidateCreate validates the OpenStackControlPlane on creation
func (r *OpenStackControlPlane) ValidateCreate(ctx context.Context, c client.Client) (admission.Warnings, error) {
	openstackcontrolplanelog.Info("validate create", "name", r.Name)

	var allWarn []string
	basePath := field.NewPath("spec")

	ctlplaneList := &OpenStackControlPlaneList{}
	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}
	if err := c.List(ctx, ctlplaneList, listOpts...); err != nil {
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

	allErrs, err := r.ValidateVersion(ctx, c)

	// Version validation can generate non-field errors, so we consider those first
	if err != nil {
		return nil, err
	}

	allWarn, errs := r.ValidateCreateServices(basePath)
	allErrs = append(allErrs, errs...)

	if err := r.ValidateTopology(basePath); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.ValidateNotificationsBusInstance(basePath); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) != 0 {
		return allWarn, apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return allWarn, nil
}

// ValidateUpdate validates the OpenStackControlPlane on update
func (r *OpenStackControlPlane) ValidateUpdate(ctx context.Context, old runtime.Object, c client.Client) (admission.Warnings, error) {
	openstackcontrolplanelog.Info("validate update", "name", r.Name)

	oldControlPlane, ok := old.(*OpenStackControlPlane)
	if !ok || oldControlPlane == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allWarn []string
	var allErrs field.ErrorList
	basePath := field.NewPath("spec")

	allWarn, errs := r.ValidateUpdateServices(oldControlPlane.Spec, basePath)
	allErrs = append(allErrs, errs...)

	if err := r.ValidateTopology(basePath); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.ValidateNotificationsBusInstance(basePath); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "core.openstack.org", Kind: "OpenStackControlPlane"},
			r.Name, allErrs)
	}

	return allWarn, nil
}

// ValidateDelete validates the OpenStackControlPlane on deletion
func (r *OpenStackControlPlane) ValidateDelete(ctx context.Context, c client.Client) (admission.Warnings, error) {
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
			reqs = "Galera, Memcached, RabbitMQ, Keystone, Glance, Neutron, Placement"
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
		// TODO(beagles): So far we haven't declared Redis as dependency for Octavia, but we might.
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
	case "Telemetry.CloudKitty":
		if !(r.Spec.Rabbitmq.Enabled && r.Spec.Keystone.Enabled) {
			reqs = "RabbitMQ, Keystone"
		}
	case "Watcher":
		if !(r.Spec.Galera.Enabled && r.Spec.Memcached.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Telemetry.Enabled && *r.Spec.Telemetry.Template.Ceilometer.Enabled &&
			*r.Spec.Telemetry.Template.MetricStorage.Enabled) {
			reqs = "Galera, Memcached, RabbitMQ, Keystone, Telemetry, Telemetry.Ceilometer, Telemetry.MetricStorage"
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
		errors = append(errors, r.Spec.Keystone.Template.ValidateCreate(basePath.Child("keystone").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Keystone.APIOverride.Route, basePath.Child("keystone").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Ironic.Enabled {
		errors = append(errors, r.Spec.Ironic.Template.ValidateCreate(basePath.Child("ironic").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Ironic.APIOverride.Route, basePath.Child("ironic").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Nova.Enabled {
		errors = append(errors, r.Spec.Nova.Template.ValidateCreate(basePath.Child("nova").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Nova.APIOverride.Route, basePath.Child("nova").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Placement.Enabled {
		errors = append(errors, r.Spec.Placement.Template.ValidateCreate(basePath.Child("placement").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Placement.APIOverride.Route, basePath.Child("placement").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Barbican.Enabled {
		errors = append(errors, r.Spec.Barbican.Template.ValidateCreate(basePath.Child("barbican").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Barbican.APIOverride.Route, basePath.Child("barbican").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Neutron.Enabled {
		errors = append(errors, r.Spec.Neutron.Template.ValidateCreate(basePath.Child("neutron").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Neutron.APIOverride.Route, basePath.Child("neutron").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Glance.Enabled {
		glanceName, _ := r.GetServiceNameCached(GlanceName, r.Spec.Glance.UniquePodNames, r.Spec.Glance.ServiceName)
		for key, glanceAPI := range r.Spec.Glance.Template.GlanceAPIs {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("glance").Child("template").Child("glanceAPIs"),
				[]string{key},
				glancev1.GetCrMaxLengthCorrection(glanceName, glanceAPI.Type)) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
		errors = append(errors, r.Spec.Glance.Template.ValidateCreate(basePath.Child("glance").Child("template"), r.Namespace)...)

		for key, override := range r.Spec.Glance.APIOverride {
			overridePath := basePath.Child("glance").Child("apiOverride").Key(key)
			errors = append(errors, validateTLSOverrideSpec(&override.Route, overridePath.Child("route"))...)
		}
	}

	if r.Spec.Cinder.Enabled {
		cinderName, _ := r.GetServiceNameCached(CinderName, r.Spec.Cinder.UniquePodNames, r.Spec.Cinder.ServiceName)
		errs := common_webhook.ValidateDNS1123Label(
			basePath.Child("cinder").Child("template").Child("cinderVolumes"),
			maps.Keys(r.Spec.Cinder.Template.CinderVolumes),
			cinderv1.GetCrMaxLengthCorrection(cinderName)) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
		errors = append(errors, errs...)
		warns, errs := r.Spec.Cinder.Template.ValidateCreate(basePath.Child("cinder").Child("template"), r.Namespace)
		errors = append(errors, errs...)
		warnings = append(warnings, warns...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Cinder.APIOverride.Route, basePath.Child("cinder").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Heat.Enabled {
		errors = append(errors, r.Spec.Heat.Template.ValidateCreate(basePath.Child("heat").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Heat.APIOverride.Route, basePath.Child("heat").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Manila.Enabled {
		errors = append(errors, r.Spec.Manila.Template.ValidateCreate(basePath.Child("manila").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Manila.APIOverride.Route, basePath.Child("manila").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Swift.Enabled {
		errors = append(errors, r.Spec.Swift.Template.ValidateCreate(basePath.Child("swift").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Swift.ProxyOverride.Route, basePath.Child("swift").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Octavia.Enabled {
		errors = append(errors, r.Spec.Octavia.Template.ValidateCreate(basePath.Child("octavia").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Octavia.APIOverride.Route, basePath.Child("octavia").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Designate.Enabled {
		errors = append(errors, r.Spec.Designate.Template.ValidateCreate(basePath.Child("designate").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Designate.APIOverride.Route, basePath.Child("designate").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Watcher.Enabled {
		errors = append(errors, r.Spec.Watcher.Template.ValidateCreate(basePath.Child("watcher").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Watcher.APIOverride.Route, basePath.Child("watcher").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Telemetry.Enabled {
		errors = append(errors, r.Spec.Telemetry.Template.ValidateCreate(basePath.Child("telemetry").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.AodhAPIOverride.Route, basePath.Child("telemetry").Child("aodhApiOverride").Child("route"))...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.PrometheusOverride.Route, basePath.Child("telemetry").Child("prometheusOverride").Child("route"))...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.AlertmanagerOverride.Route, basePath.Child("telemetry").Child("alertmanagerOverride").Child("route"))...)
	}

	// Validation for remaining services...
	if r.Spec.Galera.Enabled {
		for key, s := range *r.Spec.Galera.Templates {
			warn, err := s.ValidateCreate(basePath.Child("galera").Child("template").Key(key), r.Namespace)
			errors = append(errors, err...)
			warnings = append(warnings, warn...)
		}
	}

	if r.Spec.Memcached.Enabled {
		if r.Spec.Memcached.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("memcached").Child("templates"),
				maps.Keys(*r.Spec.Memcached.Templates),
				memcachedv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
	}

	if r.Spec.Redis.Enabled {
		if r.Spec.Redis.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("redis").Child("templates"),
				maps.Keys(*r.Spec.Redis.Templates),
				redisv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
	}

	if r.Spec.Rabbitmq.Enabled {
		if r.Spec.Rabbitmq.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("rabbitmq").Child("templates"),
				maps.Keys(*r.Spec.Rabbitmq.Templates),
				memcachedv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)

			for rabbitmqName, rabbitmqSpec := range *r.Spec.Rabbitmq.Templates {
				warn, errs := rabbitmqSpec.ValidateCreate(basePath.Child("rabbitmq").Child("template").Key(rabbitmqName), r.Namespace)
				warnings = append(warnings, warn...)
				errors = append(errors, errs...)
			}
		}
	}

	if r.Spec.Galera.Enabled {
		if r.Spec.Galera.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("galera").Child("templates"),
				maps.Keys(*r.Spec.Galera.Templates),
				mariadbv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
	}

	return warnings, errors
}

// ValidateUpdateServices validating service definitions during the OpenstackControlPlane CR update
func (r *OpenStackControlPlane) ValidateUpdateServices(old OpenStackControlPlaneSpec, basePath *field.Path) (admission.Warnings, field.ErrorList) {
	var errors field.ErrorList
	var warnings []string

	errors = append(errors, r.ValidateServiceDependencies(basePath)...)

	// Call internal validation logic for individual service operators
	if r.Spec.Keystone.Enabled {
		if old.Keystone.Template == nil {
			old.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
		}
		errors = append(errors, r.Spec.Keystone.Template.ValidateUpdate(*old.Keystone.Template, basePath.Child("keystone").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Keystone.APIOverride.Route, basePath.Child("keystone").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Ironic.Enabled {
		if old.Ironic.Template == nil {
			old.Ironic.Template = &ironicv1.IronicSpecCore{}
		}
		errors = append(errors, r.Spec.Ironic.Template.ValidateUpdate(*old.Ironic.Template, basePath.Child("ironic").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Ironic.APIOverride.Route, basePath.Child("ironic").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Nova.Enabled {
		if old.Nova.Template == nil {
			old.Nova.Template = &novav1.NovaSpecCore{}
		}
		errors = append(errors, r.Spec.Nova.Template.ValidateUpdate(*old.Nova.Template, basePath.Child("nova").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Nova.APIOverride.Route, basePath.Child("nova").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Placement.Enabled {
		if old.Placement.Template == nil {
			old.Placement.Template = &placementv1.PlacementAPISpecCore{}
		}
		errors = append(errors, r.Spec.Placement.Template.ValidateUpdate(*old.Placement.Template, basePath.Child("placement").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Placement.APIOverride.Route, basePath.Child("placement").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Barbican.Enabled {
		if old.Barbican.Template == nil {
			old.Barbican.Template = &barbicanv1.BarbicanSpecCore{}
		}
		errors = append(errors, r.Spec.Barbican.Template.ValidateUpdate(*old.Barbican.Template, basePath.Child("barbican").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Barbican.APIOverride.Route, basePath.Child("barbican").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Neutron.Enabled {
		if old.Neutron.Template == nil {
			old.Neutron.Template = &neutronv1.NeutronAPISpecCore{}
		}
		errors = append(errors, r.Spec.Neutron.Template.ValidateUpdate(*old.Neutron.Template, basePath.Child("neutron").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Neutron.APIOverride.Route, basePath.Child("neutron").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Glance.Enabled {
		if old.Glance.Template == nil {
			old.Glance.Template = &glancev1.GlanceSpecCore{}
		}
		glanceName, _ := r.GetServiceNameCached(GlanceName, r.Spec.Glance.UniquePodNames, r.Spec.Glance.ServiceName)
		for key, glanceAPI := range r.Spec.Glance.Template.GlanceAPIs {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("glance").Child("template").Child("glanceAPIs"),
				[]string{key},
				glancev1.GetCrMaxLengthCorrection(glanceName, glanceAPI.Type)) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
		errors = append(errors, r.Spec.Glance.Template.ValidateUpdate(*old.Glance.Template, basePath.Child("glance").Child("template"), r.Namespace)...)

		for key, override := range r.Spec.Glance.APIOverride {
			overridePath := basePath.Child("glance").Child("apiOverride").Key(key)
			errors = append(errors, validateTLSOverrideSpec(&override.Route, overridePath.Child("route"))...)
		}
	}

	if r.Spec.Cinder.Enabled {
		if old.Cinder.Template == nil {
			old.Cinder.Template = &cinderv1.CinderSpecCore{}
		}
		cinderName, _ := r.GetServiceNameCached(CinderName, r.Spec.Cinder.UniquePodNames, r.Spec.Cinder.ServiceName)
		errs := common_webhook.ValidateDNS1123Label(
			basePath.Child("cinder").Child("template").Child("cinderVolumes"),
			maps.Keys(r.Spec.Cinder.Template.CinderVolumes),
			cinderv1.GetCrMaxLengthCorrection(cinderName)) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
		errors = append(errors, errs...)
		warns, errs := r.Spec.Cinder.Template.ValidateUpdate(*old.Cinder.Template, basePath.Child("cinder").Child("template"), r.Namespace)
		errors = append(errors, errs...)
		warnings = append(warnings, warns...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Cinder.APIOverride.Route, basePath.Child("cinder").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Heat.Enabled {
		if old.Heat.Template == nil {
			old.Heat.Template = &heatv1.HeatSpecCore{}
		}
		errors = append(errors, r.Spec.Heat.Template.ValidateUpdate(*old.Heat.Template, basePath.Child("heat").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Heat.APIOverride.Route, basePath.Child("heat").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Manila.Enabled {
		if old.Manila.Template == nil {
			old.Manila.Template = &manilav1.ManilaSpecCore{}
		}
		errors = append(errors, r.Spec.Manila.Template.ValidateUpdate(*old.Manila.Template, basePath.Child("manila").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Manila.APIOverride.Route, basePath.Child("manila").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Swift.Enabled {
		if old.Swift.Template == nil {
			old.Swift.Template = &swiftv1.SwiftSpecCore{}
		}
		errors = append(errors, r.Spec.Swift.Template.ValidateUpdate(*old.Swift.Template, basePath.Child("swift").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Swift.ProxyOverride.Route, basePath.Child("swift").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Octavia.Enabled {
		if old.Octavia.Template == nil {
			old.Octavia.Template = &octaviav1.OctaviaSpecCore{}
		}
		errors = append(errors, r.Spec.Octavia.Template.ValidateUpdate(*old.Octavia.Template, basePath.Child("octavia").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Octavia.APIOverride.Route, basePath.Child("octavia").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Designate.Enabled {
		if old.Designate.Template == nil {
			old.Designate.Template = &designatev1.DesignateSpecCore{}
		}
		errors = append(errors, r.Spec.Designate.Template.ValidateUpdate(*old.Designate.Template, basePath.Child("designate").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Designate.APIOverride.Route, basePath.Child("designate").Child("apiOverride").Child("route"))...)
	}

	if r.Spec.Watcher.Enabled {
		if old.Watcher.Template == nil {
			old.Watcher.Template = &watcherv1.WatcherSpecCore{}
		}
		errors = append(errors, r.Spec.Watcher.Template.ValidateUpdate(*old.Watcher.Template, basePath.Child("watcher").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Watcher.APIOverride.Route, basePath.Child("watcher").Child("apiOverride").Child("route"))...)
	}
	if r.Spec.Telemetry.Enabled {
		if old.Telemetry.Template == nil {
			old.Telemetry.Template = &telemetryv1.TelemetrySpecCore{}
		}
		errors = append(errors, r.Spec.Telemetry.Template.ValidateUpdate(*old.Telemetry.Template, basePath.Child("telemetry").Child("template"), r.Namespace)...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.AodhAPIOverride.Route, basePath.Child("telemetry").Child("aodhApiOverride").Child("route"))...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.PrometheusOverride.Route, basePath.Child("telemetry").Child("prometheusOverride").Child("route"))...)
		errors = append(errors, validateTLSOverrideSpec(&r.Spec.Telemetry.AlertmanagerOverride.Route, basePath.Child("telemetry").Child("alertmanagerOverride").Child("route"))...)
	}

	if r.Spec.Memcached.Enabled {
		if r.Spec.Memcached.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("memcached").Child("templates"),
				maps.Keys(*r.Spec.Memcached.Templates),
				memcachedv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
	}

	if r.Spec.Rabbitmq.Enabled {
		if old.Rabbitmq.Templates == nil {
			old.Rabbitmq.Templates = &map[string]rabbitmqv1.RabbitMqSpecCore{}
		}
		if r.Spec.Rabbitmq.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("rabbitmq").Child("templates"),
				maps.Keys(*r.Spec.Rabbitmq.Templates),
				memcachedv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
		oldRabbitmqs := *old.Rabbitmq.Templates
		for rabbitmqName, rabbitmqSpec := range *r.Spec.Rabbitmq.Templates {
			if oldRabbitmq, ok := oldRabbitmqs[rabbitmqName]; ok {
				warn, errs := rabbitmqSpec.ValidateUpdate(oldRabbitmq, basePath.Child("rabbitmq").Child("template").Key(rabbitmqName), r.Namespace)
				warnings = append(warnings, warn...)
				errors = append(errors, errs...)
			}
		}
	}

	if r.Spec.Galera.Enabled {
		if r.Spec.Galera.Templates != nil {
			err := common_webhook.ValidateDNS1123Label(
				basePath.Child("galera").Child("templates"),
				maps.Keys(*r.Spec.Galera.Templates),
				mariadbv1.CrMaxLengthCorrection) // omit issue with statefulset pod label "controller-revision-hash": "<statefulset_name>-<hash>"
			errors = append(errors, err...)
		}
	}

	return warnings, errors
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
	if r.Spec.Watcher.Enabled {
		if depErrorMsg := r.checkDepsEnabled("Watcher"); depErrorMsg != "" {
			err := field.Invalid(basePath.Child("watcher").Child("enabled"), r.Spec.Watcher.Enabled, depErrorMsg)
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}

// ValidateVersion validates the OpenStackVersion reference in the OpenStackControlPlane
func (r *OpenStackControlPlane) ValidateVersion(ctx context.Context, c client.Client) (field.ErrorList, error) {
	var allErrs field.ErrorList

	openStackVersionList, err := GetOpenStackVersions(r.Namespace, c)

	if err != nil {
		return allErrs, apierrors.NewForbidden(
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

	if len(openStackVersionList.Items) > 0 {
		if len(openStackVersionList.Items) > 1 {
			return allErrs, apierrors.NewForbidden(
				schema.GroupResource{
					Group:    GroupVersion.WithKind("OpenStackControlPlane").Group,
					Resource: GroupVersion.WithKind("OpenStackControlPlane").Kind,
				}, r.GetName(), &field.Error{
					Type:     field.ErrorTypeForbidden,
					Field:    "",
					BadValue: r.Name,
					Detail: fmt.Sprintf(
						"multiple (%d) OpenStackVersions found in namespace %s: only one may be present.  Please rectify before creating OpenStackControlPlane",
						len(openStackVersionList.Items), r.Namespace),
				},
			)

		}

		openStackVersion := openStackVersionList.Items[0]

		if openStackVersion.Name != r.Name {
			err := field.Invalid(field.NewPath("metadata").Child("name"),
				r.Name, fmt.Sprintf("OpenStackControlPlane '%s' must have same name as the existing '%s' OpenStackVersion",
					r.Name, openStackVersion.Name))
			allErrs = append(allErrs, err)
		}
	}

	return allErrs, nil
}

// Default sets default values for the OpenStackControlPlane
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
		for k := range r.Spec.Glance.APIOverride {
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

		initializeOverrideSpec(&r.Spec.Ironic.APIOverride.Route, true)
		initializeOverrideSpec(&r.Spec.Ironic.InspectorOverride.Route, true)
		r.Spec.Ironic.Template.SetDefaultRouteAnnotations(r.Spec.Ironic.APIOverride.Route.Annotations)
		r.Spec.Ironic.Template.SetDefaultInspectorRouteAnnotations(r.Spec.Ironic.InspectorOverride.Route.Annotations)
	}

	// Keystone
	if r.Spec.Keystone.Enabled || r.Spec.Keystone.Template != nil {
		if r.Spec.Keystone.Template == nil {
			r.Spec.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
		}
		r.Spec.Keystone.Template.Default()
		initializeOverrideSpec(&r.Spec.Keystone.APIOverride.Route, true)
		r.Spec.Keystone.Template.SetDefaultRouteAnnotations(r.Spec.Keystone.APIOverride.Route.Annotations)
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
		initializeOverrideSpec(&r.Spec.Neutron.APIOverride.Route, true)
		r.Spec.Neutron.Template.SetDefaultRouteAnnotations(r.Spec.Neutron.APIOverride.Route.Annotations)
	}

	// Nova
	if r.Spec.Nova.Enabled || r.Spec.Nova.Template != nil {
		if r.Spec.Nova.Template == nil {
			r.Spec.Nova.Template = &novav1.NovaSpecCore{}
		}
		r.Spec.Nova.Template.Default()
		initializeOverrideSpec(&r.Spec.Nova.APIOverride.Route, true)
		r.Spec.Nova.Template.SetDefaultRouteAnnotations(r.Spec.Nova.APIOverride.Route.Annotations)
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
		initializeOverrideSpec(&r.Spec.Placement.APIOverride.Route, true)
		r.Spec.Placement.Template.SetDefaultRouteAnnotations(r.Spec.Placement.APIOverride.Route.Annotations)
	}

	// DNS
	if r.Spec.DNS.Enabled || r.Spec.DNS.Template != nil {
		if r.Spec.DNS.Template == nil {
			r.Spec.DNS.Template = &networkv1.DNSMasqSpecCore{}
		}

		r.Spec.DNS.Template.Default()
	}

	// Telemetry
	if r.Spec.Telemetry.Enabled || r.Spec.Telemetry.Template != nil {
		if r.Spec.Telemetry.Template == nil {
			r.Spec.Telemetry.Template = &telemetryv1.TelemetrySpecCore{}
		}
		r.Spec.Telemetry.Template.Default()
		initializeOverrideSpec(&r.Spec.Telemetry.AodhAPIOverride.Route, true)
		r.Spec.Telemetry.Template.Autoscaling.SetDefaultRouteAnnotations(r.Spec.Telemetry.AodhAPIOverride.Route.Annotations)
	}

	// Heat
	if r.Spec.Heat.Enabled || r.Spec.Heat.Template != nil {
		if r.Spec.Heat.Template == nil {
			r.Spec.Heat.Template = &heatv1.HeatSpecCore{}
		}
		r.Spec.Heat.Template.Default()
		initializeOverrideSpec(&r.Spec.Heat.APIOverride.Route, true)
		r.Spec.Heat.Template.SetDefaultRouteAnnotations(r.Spec.Heat.APIOverride.Route.Annotations)
		initializeOverrideSpec(&r.Spec.Heat.CnfAPIOverride.Route, true)
		r.Spec.Heat.Template.SetDefaultRouteAnnotations(r.Spec.Heat.CnfAPIOverride.Route.Annotations)
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
		initializeOverrideSpec(&r.Spec.Octavia.APIOverride.Route, true)
		r.Spec.Octavia.Template.SetDefaultRouteAnnotations(r.Spec.Octavia.APIOverride.Route.Annotations)
	}

	// Barbican
	if r.Spec.Barbican.Enabled || r.Spec.Barbican.Template != nil {
		if r.Spec.Barbican.Template == nil {
			r.Spec.Barbican.Template = &barbicanv1.BarbicanSpecCore{}
		}
		r.Spec.Barbican.Template.Default()
		initializeOverrideSpec(&r.Spec.Barbican.APIOverride.Route, true)
		r.Spec.Barbican.Template.SetDefaultRouteAnnotations(r.Spec.Barbican.APIOverride.Route.Annotations)
	}

	// Designate
	if r.Spec.Designate.Enabled || r.Spec.Designate.Template != nil {
		if r.Spec.Designate.Template == nil {
			r.Spec.Designate.Template = &designatev1.DesignateSpecCore{}
		}
		r.Spec.Designate.Template.Default()
	}

	// Redis
	if r.Spec.Redis.Enabled || r.Spec.Redis.Templates != nil {
		if r.Spec.Redis.Templates == nil {
			r.Spec.Redis.Templates = ptr.To(map[string]redisv1.RedisSpecCore{})
		}

		for key, template := range *r.Spec.Redis.Templates {
			template.Default()
			// By-value copy, need to update
			(*r.Spec.Redis.Templates)[key] = template
		}
	}

	// Watcher
	if r.Spec.Watcher.Enabled || r.Spec.Watcher.Template != nil {
		if r.Spec.Watcher.Template == nil {
			r.Spec.Watcher.Template = &watcherv1.WatcherSpecCore{}
		}
		r.Spec.Watcher.Template.Default()

		if r.Spec.Watcher.Enabled {
			initializeOverrideSpec(&r.Spec.Watcher.APIOverride.Route, true)
			r.Spec.Watcher.Template.SetDefaultRouteAnnotations(r.Spec.Watcher.APIOverride.Route.Annotations)
		}

		// Default DatabaseInstance
		if r.Spec.Watcher.Template.DatabaseInstance == nil || *r.Spec.Watcher.Template.DatabaseInstance == "" {
			r.Spec.Watcher.Template.DatabaseInstance = ptr.To("openstack")
		}
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

// ValidateTLSData checks if the TLS data in the apiOverride are complete
func validateTLSOverrideSpec(override **route.OverrideSpec, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if *override == nil {
		return allErrs
	}

	tlsSpec := (*override).Spec
	if tlsSpec != nil && tlsSpec.TLS != nil {
		if tlsSpec.TLS.Certificate == "" {
			allErrs = append(allErrs, field.Required(basePath.Child("tls").Child("certificate"), "Certificate is required"))
		}
		if tlsSpec.TLS.Key == "" {
			allErrs = append(allErrs, field.Required(basePath.Child("tls").Child("key"), "Key is required"))
		}
	}

	return allErrs
}

// ValidateTopology validates the TopologyRef in the OpenStackControlPlane
func (r *OpenStackControlPlane) ValidateTopology(basePath *field.Path) *field.Error {
	// When a TopologyRef CR is referenced, fail if a different Namespace is
	// referenced because is not supported
	if r.Spec.TopologyRef != nil {
		if err := topologyv1.ValidateTopologyNamespace(r.Spec.TopologyRef.Namespace, *basePath, r.Namespace); err != nil {
			return err
		}
	}
	return nil
}

// ValidateNotificationsBusInstance - returns an error if the notificationsBusInstance
// parameter is not valid.
// - nil or empty string must be raised as an error
// - when notificationsBusInstance does not point to an existing RabbitMQ instance
func (r *OpenStackControlPlane) ValidateNotificationsBusInstance(basePath *field.Path) *field.Error {
	notificationsField := basePath.Child("notificationsBusInstance")
	// no notificationsBusInstance field set, nothing to validate here
	if r.Spec.NotificationsBusInstance == nil {
		return nil
	}
	// When NotificationsBusInstance is set, fail if it is an empty string
	if *r.Spec.NotificationsBusInstance == "" {
		return field.Invalid(notificationsField, *r.Spec.NotificationsBusInstance, "notificationsBusInstance is not a valid string")
	}
	// NotificationsBusInstance is set and must be equal to an existing
	// deployed rabbitmq instance, otherwise we should fail because it
	// does not represent a valid string
	for k := range *r.Spec.Rabbitmq.Templates {
		if *r.Spec.NotificationsBusInstance == k {
			return nil
		}
	}
	return field.Invalid(notificationsField, *r.Spec.NotificationsBusInstance, "notificationsBusInstance must match an existing RabbitMQ instance name")
}
