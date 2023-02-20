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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstackcontrolplanelog = logf.Log.WithName("openstackcontrolplane-resource")

// SetupWebhookWithManager sets up the Webhook with the Manager.
func (r *OpenStackControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-core-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=Fail,sideEffects=None,groups=core.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() error {
	openstackcontrolplanelog.Info("validate create", "name", r.Name)

	return r.ValidateServices()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) error {
	openstackcontrolplanelog.Info("validate update", "name", r.Name)

	return r.ValidateServices()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() error {
	openstackcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *OpenStackControlPlane) checkDepsEnabled(name string) bool {
	switch name {
	case "Keystone":
		return r.Spec.Mariadb.Enabled
	case "Glance":
		return r.Spec.Mariadb.Enabled && r.Spec.Keystone.Enabled
	case "Cinder":
		return (r.Spec.Mariadb.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled)
	case "Placement":
		return r.Spec.Mariadb.Enabled && r.Spec.Keystone.Enabled
	case "Neutron":
		return (r.Spec.Mariadb.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled)
	case "Nova":
		return (r.Spec.Mariadb.Enabled && r.Spec.Rabbitmq.Enabled &&
			r.Spec.Keystone.Enabled && r.Spec.Placement.Enabled &&
			r.Spec.Neutron.Enabled && r.Spec.Glance.Enabled)
	}
	return true
}

// ValidateServices implements common function for validating services
func (r *OpenStackControlPlane) ValidateServices() error {

	// Add service dependency validations
	errorMsg := "%s service dependencies are not enabled."

	if r.Spec.Keystone.Enabled && !r.checkDepsEnabled("Keystone") {
		return fmt.Errorf(errorMsg, "Keystone")
	}

	if r.Spec.Glance.Enabled && !r.checkDepsEnabled("Glance") {
		return fmt.Errorf(errorMsg, "Glance")
	}

	if r.Spec.Cinder.Enabled && !r.checkDepsEnabled("Cinder") {
		return fmt.Errorf(errorMsg, "Cinder")
	}

	if r.Spec.Placement.Enabled && !r.checkDepsEnabled("Placement") {
		return fmt.Errorf(errorMsg, "Placement")
	}

	if r.Spec.Neutron.Enabled && !r.checkDepsEnabled("Neutron") {
		return fmt.Errorf(errorMsg, "Neutron")
	}

	if r.Spec.Nova.Enabled && !r.checkDepsEnabled("Nova") {
		return fmt.Errorf(errorMsg, "Nova")
	}

	return nil
}
