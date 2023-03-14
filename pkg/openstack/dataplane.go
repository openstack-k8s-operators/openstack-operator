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

package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dataplanev1beta1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileDataPlane -
func ReconcileDataPlane(ctx context.Context, instance *corev1beta1.OpenStackDataPlane, helper *helper.Helper) (ctrl.Result, error) {
	// roles and nodes may depend on each other, so we run both before checking for errors
	roleReady, roleErr := ReconcileDataPlaneRole(ctx, instance, helper)
	nodeReady, nodeErr := ReconcileDataPlaneNode(ctx, instance, helper)
	if roleErr != nil {
		return ctrl.Result{}, roleErr
	}

	// Checking role for readiness/errors
	if roleReady {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackDataPlaneRoleReadyCondition, corev1beta1.OpenStackDataPlaneRoleReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackDataPlaneRoleReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackDataPlaneRoleReadyRunningMessage))
	}

	// Checking node for readiness/errors
	if nodeErr != nil {
		return ctrl.Result{}, nodeErr
	}
	if nodeReady {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackDataPlaneNodeReadyCondition, corev1beta1.OpenStackDataPlaneNodeReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackDataPlaneNodeReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackDataPlaneNodeReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// ReconcileDataPlaneNode -
func ReconcileDataPlaneNode(ctx context.Context, instance *corev1beta1.OpenStackDataPlane, helper *helper.Helper) (bool, error) {
	ready := true
	node := &dataplanev1beta1.OpenStackDataPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: instance.Namespace,
		},
	}

	for nodeName, nodeSpec := range instance.Spec.Nodes {
		node.Name = nodeName
		nodeSpec.DeepCopyInto(&node.Spec)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), node, func() error {
			helper.GetLogger().Info("Reconciling Node", "Node.Namespace", instance.Namespace, "Node.Name", node.Name)
			err := controllerutil.SetControllerReference(helper.GetBeforeObject(), node, helper.GetScheme())
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackDataPlaneNodeReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				corev1beta1.OpenStackDataPlaneNodeReadyErrorMessage,
				err.Error()))
			return false, err
		}
		if op != controllerutil.OperationResultNone {
			helper.GetLogger().Info(fmt.Sprintf("Node %s - %s", node.Name, op))
		}

		ready = ready && node.IsReady()
	}

	return ready, nil
}

// ReconcileDataPlaneRole -
func ReconcileDataPlaneRole(ctx context.Context, instance *corev1beta1.OpenStackDataPlane, helper *helper.Helper) (bool, error) {
	ready := true
	role := &dataplanev1beta1.OpenStackDataPlaneRole{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: instance.Namespace,
		},
	}
	for roleName, roleSpec := range instance.Spec.Roles {
		role.Name = roleName
		roleSpec.DeepCopyInto(&role.Spec)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), role, func() error {
			helper.GetLogger().Info("Reconciling Role", "Role.Namespace", instance.Namespace, "Role.Name", role.Name)
			err := controllerutil.SetControllerReference(helper.GetBeforeObject(), role, helper.GetScheme())
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackDataPlaneRoleReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				corev1beta1.OpenStackDataPlaneRoleReadyErrorMessage,
				err.Error()))
			return false, err
		}
		if op != controllerutil.OperationResultNone {
			helper.GetLogger().Info(fmt.Sprintf("Role %s - %s", role.Name, op))
		}

		ready = ready && role.IsReady()

	}
	return ready, nil
}
