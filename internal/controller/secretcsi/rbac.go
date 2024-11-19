package secretcsi

import (
	"context"
	"time"

	operatorclient "github.com/zncdatadev/operator-go/pkg/client"
	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
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

	sa := r.buildServiceAccount()

	if mutant, err := operatorclient.CreateOrUpdate(ctx, r.client, sa); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	clusterRole := r.buildClusterRole()

	if mutant, err := operatorclient.CreateOrUpdate(ctx, r.client, clusterRole); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	clusterRoleBinding, err := r.descClusterRoleBinding(ctx)
	if err != nil && client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	clusterRoleBinding, err = r.clusterRoleBindingGC(ctx, clusterRoleBinding)
	if err != nil {
		return ctrl.Result{}, err
	}

	clusterRoleBinding = r.buildClusterRoleBinding(clusterRoleBinding)

	if mutant, err := operatorclient.CreateOrUpdate(ctx, r.client, clusterRoleBinding); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil

}

func (r *RBAC) buildServiceAccount() *corev1.ServiceAccount {

	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CSIServiceAccountName,
			Namespace: r.cr.GetNamespace(),
		},
	}
	return obj
}

func (r *RBAC) buildClusterRole() *rbacv1.ClusterRole {
	obj := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSIClusterRoleName,
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
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch", "patch"},
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
			{
				APIGroups: []string{"secrets.kubedoop.dev"},
				Resources: []string{"secretclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	return obj
}

func (r *RBAC) descClusterRoleBinding(ctx context.Context) (*rbacv1.ClusterRoleBinding, error) {
	obj := &rbacv1.ClusterRoleBinding{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name: CSIClusterRoleBindingName,
	}, obj); err != nil {
		return nil, err
	}

	return obj.DeepCopy(), nil
}

func (r *RBAC) buildClusterRoleBinding(existRoleBinding *rbacv1.ClusterRoleBinding) *rbacv1.ClusterRoleBinding {
	obj := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: CSIClusterRoleBindingName,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef:  rbacv1.RoleRef{},
	}

	if existRoleBinding != nil {
		obj = existRoleBinding.DeepCopy()
		logger.V(1).Info("found exist cluster role binding", "name", obj.Name, "subjectLength", len(obj.Subjects))
	}

	obj.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     CSIClusterRoleName,
		APIGroup: "rbac.authorization.k8s.io",
	}

	alreadyBinding := false
	for _, subj := range obj.Subjects {
		if subj.Kind == "ServiceAccount" && subj.Name == CSIServiceAccountName && subj.Namespace == r.cr.GetNamespace() {
			alreadyBinding = true
			break
		}
	}

	if !alreadyBinding {
		obj.Subjects = append(obj.Subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      CSIServiceAccountName,
			Namespace: r.cr.GetNamespace(),
		})
	}

	return obj
}

// clusterRoleBindingGC remove service account not found
func (r *RBAC) clusterRoleBindingGC(ctx context.Context, obj *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {

	if obj == nil {
		return nil, nil
	}

	obj = obj.DeepCopy()
	filterSubjs := []rbacv1.Subject{}
	for i, subj := range obj.Subjects {
		if subj.Kind == "ServiceAccount" {
			if exist, err := r.getServiceAccount(ctx, subj.Name, subj.Namespace); err != nil {
				return nil, err
			} else if exist {
				filterSubjs = append(filterSubjs, obj.Subjects[i])
			} else {
				logger.V(1).Info("service account not found", "name", subj.Name, "namespace", subj.Namespace)
			}
		}
	}

	obj.Subjects = filterSubjs

	return obj, nil
}

func (r *RBAC) getServiceAccount(ctx context.Context, name, namespace string) (bool, error) {
	obj := &corev1.ServiceAccount{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}, obj); err != nil {
		// notfound
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
