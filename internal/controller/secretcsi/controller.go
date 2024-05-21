/*
Copyright 2024 zncdatadev.

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

package secret_csi_plugin

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
)

const (
	VOLUMES_MOUNTPOINT_DIR_NAME   = "mountpoint-dir"
	VOLUMES_PLUGIN_DIR_NAME       = "plugin-dir"
	VOLUMES_REGISTRATION_DIR_NAME = "registration-dir"
)

var (
	logger = ctrl.Log.WithName("secret controller")
)

// SecretCSIReconciler reconciles a SecretCSI object
type SecretCSIReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=secrets.zncdata.dev,resources=secretcsis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=secrets.zncdata.dev,resources=secretcsis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=secrets.zncdata.dev,resources=secretcsis/finalizers,verbs=update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretCSI object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *SecretCSIReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	instance := &secretsv1alpha1.SecretCSI{}

	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.V(5).Info("SecretCSI resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SecretCSI")
		return ctrl.Result{}, err
	}

	logger.V(1).Info("Reconciling SecretCSI", "Name", instance.Name)

	if result, err := NewCSIDriver(r.Client, instance).Reconcile(ctx); err != nil {
		return result, err
	} else if result.Requeue {
		return result, nil
	}

	if result, err := NewRBAC(r.Client, instance).Reconcile(ctx); err != nil {
		return result, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	if result, err := NewStorageClass(r.Client, instance).Reconcile(ctx); err != nil {
		return result, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	daemonSet := NewDaemonSet(r.Client, instance, &instance.Spec, CSIServiceAccountName)

	if result, err := daemonSet.Reconcile(ctx); err != nil {
		return result, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretCSIReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretsv1alpha1.SecretCSI{}).
		Complete(r)
}
