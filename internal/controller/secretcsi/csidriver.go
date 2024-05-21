package secret_csi_plugin

import (
	"context"
	"time"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/resource"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CSIDriver struct {
	client client.Client

	cr *secretsv1alpha1.SecretCSI
}

func NewCSIDriver(client client.Client, cr *secretsv1alpha1.SecretCSI) *CSIDriver {
	return &CSIDriver{
		client: client,
		cr:     cr,
	}
}

func (r *CSIDriver) Reconcile(ctx context.Context) (ctrl.Result, error) {

	obj := r.build()

	return r.apply(ctx, obj)

}

func (r *CSIDriver) build() *storage.CSIDriver {
	attachRequired := false
	podInfoOnMount := true

	obj := &storage.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: "secrets.zncdata.dev",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "secret-operator",
			},
		},
		Spec: storage.CSIDriverSpec{
			AttachRequired: &attachRequired,
			PodInfoOnMount: &podInfoOnMount,
			VolumeLifecycleModes: []storage.VolumeLifecycleMode{
				storage.VolumeLifecyclePersistent,
				storage.VolumeLifecycleEphemeral,
			},
		},
	}

	return obj

}

func (r *CSIDriver) apply(ctx context.Context, obj *storage.CSIDriver) (ctrl.Result, error) {
	if mutant, err := resource.CreateOrUpdate(ctx, r.client, obj); err != nil {
		return ctrl.Result{}, err
	} else if mutant {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return ctrl.Result{}, nil

}
