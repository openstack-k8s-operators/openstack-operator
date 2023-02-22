package openstack

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureDeleted - Delete the object which in turn will clean the sub resources
func EnsureDeleted(ctx context.Context, helper *helper.Helper, obj client.Object) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := helper.GetClient().Get(ctx, key, obj); err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Delete the object
	if obj.GetDeletionTimestamp().IsZero() {
		if err := helper.GetClient().Delete(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil

}
