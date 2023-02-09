package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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

// CheckDeleteSubresources - Delete all resources of the []client.ObjectList interface slice passed as "subResLists" and
// return a requeue result if anything in those lists is still waiting to be fully deleted.  Before iterating, however,
// any empty client.ObjectList passed within "subResLists" will be populated with any existing resources of that
// client.ObjectList's actual type (i.e. if the specific type is glancev1.GlanceList then all glancev1.Glance resources
// in the namespace will be queried and injected into the client.ObjectList interface for it).
func CheckDeleteSubresources(ctx context.Context, helper *helper.Helper, subResLists []client.ObjectList) (ctrl.Result, error) {
	found := false
	res := ctrl.Result{}
	ctlPlaneObj := helper.GetBeforeObject()

	// First get lists of any actual subresources for all the subresource types that interest us *if* that
	// subresource type list is empty (i.e. was not passed pre-populated with subresources)
	for _, subResList := range subResLists {
		if apimeta.LenList(subResList) == 0 {
			// Attempt to populate the list if it was included but empty
			if err := helper.GetClient().List(ctx, subResList, &client.ListOptions{Namespace: ctlPlaneObj.GetNamespace()}); err != nil {
				return res, err
			}

			// The list (subResLists[index]) will now be populated if anything was found
		}
	}

	// Declare a walker func for the individual subresources of the list of subresource lists
	// This func will delete the subresource parameter and flag that removal of subresources is still in progress
	checkDeleteSubresource := func(o runtime.Object) error {
		var obj client.Object
		var ok bool

		if obj, ok = o.(client.Object); !ok {
			err := fmt.Errorf("unable to convert runtime %v into client.Object", o)
			util.LogErrorForObject(helper, err, err.Error(), ctlPlaneObj)
			return err
		}

		for _, ownerRef := range obj.GetOwnerReferences() {
			if ownerRef.Kind == ctlPlaneObj.GetObjectKind().GroupVersionKind().Kind && ownerRef.Name == ctlPlaneObj.GetName() {
				if obj.GetDeletionTimestamp().IsZero() {
					if err := helper.GetClient().Delete(ctx, obj); err != nil {
						return err
					}
				}

				found = true
				util.LogForObject(helper, fmt.Sprintf("OpenStackControlPlane %s deletion waiting for deletion of %s %s", ctlPlaneObj.GetName(), obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()), ctlPlaneObj)
			}
		}

		return nil
	}

	// For each list of subresources, call the walker func (which will iterate each item
	// in the particular subresource list)
	for _, subResList := range subResLists {
		if err := apimeta.EachListItem(subResList, checkDeleteSubresource); err != nil {
			return res, err
		}
	}

	// If any subresources were found (and thus not fully deleted yet), we indicate that
	// we need a requeue
	if found {
		res = ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}
	}

	return res, nil
}
