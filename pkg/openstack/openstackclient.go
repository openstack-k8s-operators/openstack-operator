/*

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

	clientv1beta1 "github.com/openstack-k8s-operators/infra-operator/apis/client/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ServiceAccount -
	ServiceAccount = "openstack-operator-openstackclient"
)

// ReconcileOpenStackClient -
func ReconcileOpenStackClient(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper, openstackClientImage string) (ctrl.Result, error) {

	openstackclient := &clientv1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling OpenStackClient", "OpenStackClient.Namespace", instance.Namespace, "OpenStackClient.Name", openstackclient.Name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), openstackclient, func() error {
		openstackclient.Spec.ContainerImage = openstackClientImage

		// the following are created/owned by keystoneclient
		openstackclient.Spec.OpenStackConfigMap = "openstack-config"
		openstackclient.Spec.OpenStackConfigSecret = "openstack-config-secret"
		openstackclient.Spec.NodeSelector = instance.Spec.NodeSelector

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), openstackclient, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("OpenStackClient %s - %s", openstackclient.Name, op))
	}

	if openstackclient.Status.Conditions.IsTrue(clientv1beta1.OpenStackClientReadyCondition) {
		helper.GetLogger().Info("OpenStackClient ready condition is true")
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneClientReadyCondition, corev1beta1.OpenStackControlPlaneClientReadyMessage)
	} else {
		helper.GetLogger().Info("OpenStackClient ready condition is false")
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneClientReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneClientReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
