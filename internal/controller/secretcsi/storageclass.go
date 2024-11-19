package secretcsi

import (
	"context"
	"time"

	operatorclient "github.com/zncdatadev/operator-go/pkg/client"
	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StorageClass struct {
	client client.Client

	cr *secretsv1alpha1.SecretCSI
}

func NewStorageClass(client client.Client, cr *secretsv1alpha1.SecretCSI) *StorageClass {
	return &StorageClass{
		client: client,
		cr:     cr,
	}
}

func (r *StorageClass) Reconcile(ctx context.Context) (ctrl.Result, error) {

	obj := r.build()

	return r.apply(ctx, obj)
}

func (r *StorageClass) build() *storage.StorageClass {

	obj := &storage.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secrets.kubedoop.dev",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "secret-operator",
			},
		},
		Provisioner: "secrets.kubedoop.dev",
	}

	return obj
}

func (r *StorageClass) apply(ctx context.Context, obj *storage.StorageClass) (ctrl.Result, error) {
	mutant, err := operatorclient.CreateOrUpdate(ctx, r.client, obj)
	if err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
	return ctrl.Result{}, nil
}
