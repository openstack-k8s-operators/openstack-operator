/*
Copyright 2023.

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
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var openstackdataplanenodesetlog = logf.Log.WithName("openstackdataplanenodeset-resource")

// Default sets default values for the OpenStackDataPlaneNodeSet
func (r *OpenStackDataPlaneNodeSet) Default() {
	openstackdataplanenodesetlog.Info("default", "name", r.Name)
	r.Spec.Default()
}

// Default - set defaults for this OpenStackDataPlaneNodeSet Spec
func (spec *OpenStackDataPlaneNodeSetSpec) Default() {
	var domain string
	if spec.BaremetalSetTemplate != nil {
		domain = spec.BaremetalSetTemplate.DomainName
	}

	for nodeName, node := range spec.Nodes {
		if node.HostName == "" {
			node.HostName = nodeName
		}
		if !spec.PreProvisioned {
			if !NodeHostNameIsFQDN(node.HostName) && domain != "" {
				node.HostName = strings.Join([]string{nodeName, domain}, ".")
			}
		}

		spec.Nodes[nodeName] = *node.DeepCopy()
	}

	if !spec.PreProvisioned && spec.BaremetalSetTemplate != nil {
		spec.NodeTemplate.Ansible.AnsibleUser = spec.BaremetalSetTemplate.CloudUserName
		if spec.BaremetalSetTemplate.DeploymentSSHSecret == "" {
			spec.BaremetalSetTemplate.DeploymentSSHSecret = spec.NodeTemplate.AnsibleSSHPrivateKeySecret
		}
	} else if spec.NodeTemplate.Ansible.AnsibleUser == "" {
		spec.NodeTemplate.Ansible.AnsibleUser = "cloud-admin"
	}
}

// ValidateCreate validates the OpenStackDataPlaneNodeSet on creation
func (r *OpenStackDataPlaneNodeSet) ValidateCreate(ctx context.Context, c client.Client) (admission.Warnings, error) {
	openstackdataplanenodesetlog.Info("validate create", "name", r.Name)
	var errors field.ErrorList
	errors, err := r.validateNodes(ctx, c)
	if err != nil {
		return nil, err
	}
	// Check if OpenStackDataPlaneNodeSet name matches RFC1123 for use in labels
	validate := validator.New()
	if err := validate.Var(r.Name, "hostname_rfc1123"); err != nil {
		openstackdataplanenodesetlog.Error(err, "Error validating OpenStackDataPlaneNodeSet name, name must follow RFC1123")
		errors = append(errors, field.Invalid(
			field.NewPath("Name"),
			r.Name,
			fmt.Sprintf("Error validating OpenStackDataPlaneNodeSet name %s, name must follow RFC1123", r.Name)))
	}
	// Validate volume names
	for _, emount := range r.Spec.NodeTemplate.ExtraMounts {
		for _, vol := range emount.Volumes {
			msgs := apimachineryvalidation.IsDNS1123Label(vol.Name)
			for _, msg := range msgs {
				errors = append(errors, field.Invalid(
					field.NewPath("spec.nodeTemplate.extraMounts"),
					vol.Name,
					msg))
			}
		}
	}
	if len(errors) > 0 {
		openstackdataplanenodesetlog.Info("validation failed", "name", r.Name)

		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneNodeSet"},
			r.Name,
			errors)
	}
	return nil, nil
}

func (r *OpenStackDataPlaneNodeSet) validateNodes(ctx context.Context, c client.Client) (field.ErrorList, error) {
	var errors field.ErrorList
	nodeSetList := &OpenStackDataPlaneNodeSetList{}
	opts := &client.ListOptions{
		Namespace: r.ObjectMeta.Namespace,
	}

	err := c.List(ctx, nodeSetList, opts)
	if err != nil {
		return nil, err
	}

	// Currently, this check is only valid for PreProvisioned nodes. Since we can't possibly
	// have duplicates in Baremetal Deployments, we can exit early here for Baremetal NodeSets.
	// If this is the first NodeSet being created, then there can be no duplicates
	// we can exit early here.
	if r.Spec.PreProvisioned && len(nodeSetList.Items) != 0 {
		errors = append(errors, r.Spec.duplicateNodeCheck(nodeSetList, r.ObjectMeta.Name)...)
	}

	return errors, nil

}

// ValidateUpdate validates the OpenStackDataPlaneNodeSet on update
func (r *OpenStackDataPlaneNodeSet) ValidateUpdate(ctx context.Context, old runtime.Object, c client.Client) (admission.Warnings, error) {
	openstackdataplanenodesetlog.Info("validate update", "name", r.Name)
	oldNodeSet, ok := old.(*OpenStackDataPlaneNodeSet)
	if !ok {
		return nil, apierrors.NewInternalError(
			fmt.Errorf("expected a OpenStackDataPlaneNodeSet object, but got %T", oldNodeSet))
	}
	errors, err := r.validateNodes(ctx, c)
	if err != nil {
		return nil, err
	}

	errors = append(errors, r.Spec.ValidateUpdate(&oldNodeSet.Spec)...)

	if errors != nil {
		openstackdataplanenodesetlog.Info("validation failed", "name", r.Name)
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneNodeSet"},
			r.Name,
			errors,
		)

	}
	if oldNodeSet.Status.DeploymentStatuses != nil {
		for deployName, deployConditions := range oldNodeSet.Status.DeploymentStatuses {
			deployCondition := deployConditions.Get(NodeSetDeploymentReadyCondition)
			if !deployConditions.IsTrue(NodeSetDeploymentReadyCondition) && !condition.IsError(deployCondition) {
				return nil, apierrors.NewConflict(
					schema.GroupResource{Group: "dataplane.openstack.org", Resource: "OpenStackDataPlaneNodeSet"},
					r.Name,
					fmt.Errorf("could not patch openstackdataplanenodeset while openstackdataplanedeployment %s (blocked on %s condition) is running",
						deployName, string(deployCondition.Type)),
				)
			}
		}
	}

	return nil, nil
}

// ValidateUpdate validates the OpenStackDataPlaneNodeSetSpec on update
func (spec *OpenStackDataPlaneNodeSetSpec) ValidateUpdate(oldSpec *OpenStackDataPlaneNodeSetSpec) field.ErrorList {

	var errors field.ErrorList
	// Some changes to the baremetalSetTemplate after the initial deployment would necessitate
	// a redeploy of the node. Thus we should block these changes and require the user to
	// delete and redeploy should they wish to make such changes after the initial deploy.
	// If the BaremetalSetTemplate is changed, we will offload the parsing of these details
	// to the openstack-baremetal-operator webhook to avoid duplicating logic.
	if !reflect.DeepEqual(spec.BaremetalSetTemplate, oldSpec.BaremetalSetTemplate) {
		// Call openstack-baremetal-operator webhook Validate() to parse changes
		if spec.BaremetalSetTemplate != nil && oldSpec.BaremetalSetTemplate != nil {
			err := spec.BaremetalSetTemplate.ValidateTemplate(
				len(oldSpec.Nodes), *oldSpec.BaremetalSetTemplate)
			if err != nil {
				errors = append(errors, field.Forbidden(
					field.NewPath("spec.baremetalSetTemplate"),
					fmt.Sprintf("%s", err)))
			}
		}
	}

	return errors
}

// ValidateDelete validates the OpenStackDataPlaneNodeSet on deletion
func (r *OpenStackDataPlaneNodeSet) ValidateDelete(ctx context.Context, c client.Client) (admission.Warnings, error) {
	openstackdataplanenodesetlog.Info("validate delete", "name", r.Name)
	errors := r.Spec.ValidateDelete()

	if len(errors) != 0 {
		openstackdataplanenodesetlog.Info("validation failed", "name", r.Name)

		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "dataplane.openstack.org", Kind: "OpenStackDataPlaneNodeSet"},
			r.Name,
			errors,
		)
	}
	return nil, nil
}

// ValidateDelete validates the OpenStackDataPlaneNodeSetSpec on delete
func (spec *OpenStackDataPlaneNodeSetSpec) ValidateDelete() field.ErrorList {
	// TODO(user): fill in your validation logic upon object deletion.

	return field.ErrorList{}

}
