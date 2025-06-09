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

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	lightspeedv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/lightspeed/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileLightSpeed - reconciles OpenStackLightspeed
func ReconcileLightSpeed(ctx context.Context, instance *corev1.OpenStackControlPlane, version *corev1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)
	lightspeed := &lightspeedv1beta1.OpenStackLightspeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstack-lightspeed",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.OpenStackLightspeed.Enabled {
		if res, err := EnsureDeleted(ctx, helper, lightspeed); err != nil {
			return res, err
		}

		return ctrl.Result{}, nil
	}

	if instance.Spec.OpenStackLightspeed.Template == nil {
		return ctrl.Result{}, fmt.Errorf("OpenStackLightspeed template is required")
	}

	Log.Info("Reconciling OpenStackLightspeed", "OpenStackLightspeed.Namespace", instance.Namespace, "OpenStackLightspeed.Name", lightspeed.Name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), lightspeed, func() error {
		instance.Spec.OpenStackLightspeed.Template.DeepCopyInto(&lightspeed.Spec.OpenStackLightspeedCore)

		if version.Status.ContainerImages.OpenstackLightspeedImage == nil {
			return fmt.Errorf("OpenstackLightspeedImage value is missing")
		}
		lightspeed.Spec.RAGImage = *version.Status.ContainerImages.OpenstackLightspeedImage

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), lightspeed, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		Log.Error(err, "Failed to reconcile OpenStackLightspeed")
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneLightspeedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneLightSpeedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OpenStackLightspeed %s - %s", instance.Name, op))
	}

	if lightspeed.Status.ObservedGeneration == lightspeed.Generation && lightspeed.IsReady() {
		Log.Info("OpenStackLightspeed ready condition is true")
		instance.Status.ContainerImages.OpenstackLightspeedImage = version.Status.ContainerImages.OpenstackLightspeedImage
		instance.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneLightspeedReadyCondition, corev1.OpenStackControlPlaneLightSpeedReadyMessage)
	} else {
		Log.Info("OpenStackLightspeed ready condition is false")
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneLightspeedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneLightSpeedReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

// LightspeedImageMatch - return true if the openstacklightspeed images match on the ControlPlane and Version, or if OpenStackLightspeed is not enabled
func LightspeedImageMatch(ctx context.Context, controlPlane *corev1.OpenStackControlPlane, version *corev1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.OpenStackLightspeed.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.OpenstackLightspeedImage, version.Status.ContainerImages.OpenstackLightspeedImage) {
			Log.Info("OpenStackLightspeed vector DB images do not match")
			return false
		}

	}

	return true
}
