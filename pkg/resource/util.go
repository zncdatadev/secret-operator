package resource

import (
	"context"

	"github.com/cisco-open/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("resource-util")
)

func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	logger = ctrl.Log.WithName("util")
	key := client.ObjectKeyFromObject(obj)
	namespace := obj.GetNamespace()

	kinds, _, _ := scheme.Scheme.ObjectKinds(obj)

	name := obj.GetName()
	current := obj.DeepCopyObject().(client.Object)
	// Check if the object exists, if not create a new one
	err := c.Get(ctx, key, current)
	if errors.IsNotFound(err) {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj); err != nil {
			return false, err
		}
		logger.Info("Creating a new object", "Kind", kinds, "Namespace", namespace, "Name", name)

		if err := c.Create(ctx, obj); err != nil {
			return false, err
		}
		return true, nil
	} else if err == nil {
		switch obj.(type) {
		case *corev1.Service:
			currentSvc := current.(*corev1.Service)
			svc := obj.(*corev1.Service)
			// Preserve the ClusterIP when updating the service
			svc.Spec.ClusterIP = currentSvc.Spec.ClusterIP
			// Preserve the annotation when updating the service, ensure any updated annotation is preserved
			//for key, value := range currentSvc.Annotations {
			//	if _, present := svc.Annotations[key]; !present {
			//		svc.Annotations[key] = value
			//	}
			//}

			if svc.Spec.Type == corev1.ServiceTypeNodePort || svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				for i := range svc.Spec.Ports {
					svc.Spec.Ports[i].NodePort = currentSvc.Spec.Ports[i].NodePort
				}
			}
		}
		result, err := patch.DefaultPatchMaker.Calculate(current, obj, patch.IgnoreStatusFields())
		if err != nil {
			logger.Error(err, "failed to calculate patch to match objects, moving on to update")
			// if there is an error with matching, we still want to update
			resourceVersion := current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion()
			obj.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(resourceVersion)

			if err := c.Update(ctx, obj); err != nil {
				return false, err
			}
			return true, nil
		}

		if !result.IsEmpty() {
			logger.V(2).Info("Resource update object", "kind", kinds, "name", obj.(metav1.ObjectMetaAccessor).GetObjectMeta().GetName(), "patch", string(result.Patch))

			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(obj); err != nil {
				logger.Error(err, "failed to annotate modified object", "object", obj)
			}

			resourceVersion := current.(metav1.ObjectMetaAccessor).GetObjectMeta().GetResourceVersion()
			obj.(metav1.ObjectMetaAccessor).GetObjectMeta().SetResourceVersion(resourceVersion)

			if err = c.Update(ctx, obj); err != nil {
				return false, err
			}
			return true, nil
		}

		logger.V(2).Info("Skipping update for object", "kind", kinds, "name", obj.(metav1.ObjectMetaAccessor).GetObjectMeta().GetName())

	}
	return false, err
}
