package secret_csi_plugin

import (
	"context"
	"time"

	secretsv1alpha1 "github.com/zncdata-labs/secret-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RBAC struct {
	client client.Client

	cr *secretsv1alpha1.SecretCSI
}

func NewRBAC(client client.Client, cr *secretsv1alpha1.SecretCSI) *RBAC {
	return &RBAC{
		client: client,
		cr:     cr,
	}
}

func (r *RBAC) Reconcile(ctx context.Context) (ctrl.Result, error) {

	return r.apply(ctx)
}

func (r *RBAC) apply(ctx context.Context) (ctrl.Result, error) {

	sa, clusterRole, clusterRoleBinding := r.build()

	if err := ctrl.SetControllerReference(r.cr, sa, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}

	if err := ctrl.SetControllerReference(r.cr, clusterRole, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}

	if err := ctrl.SetControllerReference(r.cr, clusterRoleBinding, r.client.Scheme()); err != nil {
		return ctrl.Result{}, err
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, sa); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, clusterRole); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	if mutant, err := CreateOrUpdate(ctx, r.client, clusterRoleBinding); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil

}

func (r *RBAC) build() (*corev1.ServiceAccount, *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding) {

	sa := r.buildServiceAccount()
	clusterRole := r.buildClusterRole()

	clusterRoleBinding := r.buildClusterRoleBinding()

	return sa, clusterRole, clusterRoleBinding
}

func (r *RBAC) buildServiceAccount() *corev1.ServiceAccount {

	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSI_SERVICEACCOUNT_NAME,
		},
	}
	return obj
}

func (r *RBAC) buildClusterRole() *rbacv1.ClusterRole {
	obj := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSI_CLUSTERROLE_NAME,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch", "create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"csidrivers"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	return obj
}

func (r *RBAC) buildClusterRoleBinding() *rbacv1.ClusterRoleBinding {

	obj := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSI_CLUSTERROLEBINDING_NAME,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      CSI_SERVICEACCOUNT_NAME,
				Namespace: r.cr.GetNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     CSI_CLUSTERROLE_NAME,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return obj
}
