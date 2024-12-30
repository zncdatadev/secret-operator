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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	secretvs1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
)

// SecretClassReconciler reconciles a SecretClass object
type SecretClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=secrets.kubedoop.dev,resources=secretclasses/finalizers,verbs=update
// +kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=persistentvolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=listeners.kubedoop.dev,resources=listeners,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretClass object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *SecretClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&secretvs1alpha1.SecretClass{}).
		Complete(r)
}
