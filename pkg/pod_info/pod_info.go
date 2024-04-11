package pod_info

import (
	"context"
	"fmt"
	"net"

	listenersv1alpha1 "github.com/zncdata-labs/listener-operator/api/v1alpha1"
	listenerUtil "github.com/zncdata-labs/listener-operator/pkg/util"
	"github.com/zncdata-labs/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("pod-info")
)

type PodInfo struct {
	client         client.Client
	Pod            *corev1.Pod
	VolumeSelector *volume.SecretVolumeSelector
}

func NewPodInfo(
	client client.Client,
	pod *corev1.Pod,
	volumeSelector *volume.SecretVolumeSelector,
) *PodInfo {
	return &PodInfo{
		Pod:            pod,
		VolumeSelector: volumeSelector,
	}
}

func (p *PodInfo) GetPodName() string {
	return p.Pod.GetName()
}

func (p *PodInfo) GetPodNamespace() string {
	return p.Pod.GetNamespace()
}

func (p *PodInfo) GetPodIP() string {
	return p.Pod.Status.PodIP
}

func (p *PodInfo) GetPodIPs() []string {
	ips := []string{}
	for _, address := range p.Pod.Status.PodIPs {
		ips = append(ips, address.IP)
	}
	return ips
}

func (p *PodInfo) GetNodeName() string {
	return p.Pod.Spec.NodeName
}

func (p *PodInfo) GetNode(ctx context.Context) (*corev1.Node, error) {
	nodeName := p.GetNodeName()
	node := &corev1.Node{}
	err := p.client.Get(
		ctx,
		client.ObjectKey{
			Name: nodeName,
		},
		node,
	)
	if err != nil {
		return nil, err
	}

	return node, nil

}

func (p *PodInfo) GetNodeIPs(ctx context.Context) []Address {

	node, err := p.GetNode(ctx)
	if err != nil {
		return nil
	}

	addresses := []Address{}

	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
			addresses = append(addresses, Address{
				IP: net.IP(address.Address),
			})
		}
	}

	return addresses
}

func (p *PodInfo) GetServiceIPsByName(name string) []Address {
	addresses := []Address{
		{
			Hostname: fmt.Sprintf("%s.%s.svc.cluster.local", name, p.GetPodNamespace()),
		},
	}

	return addresses
}

// Get the address information of the pod.
// In statusfulset, the spec.serviceName field is required, so the pod will come with pod.spec.subdomain.
// In deployment, the pod does not have pod.spec.subdomain by default. If needed, you can first create a Service, and then
// configure the subdomain field for the podTemplate in the deployment.
func (p *PodInfo) GetPodAddresses() []Address {
	svcName := p.Pod.Spec.Subdomain
	if svcName == "" {
		return nil
	}

	// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
	addresses := []Address{
		{
			Hostname: fmt.Sprintf("%s.%s.svc.cluster.local", svcName, p.GetPodNamespace()),
		},
		{
			Hostname: fmt.Sprintf("%s.%s.%s.svc.cluster.local", p.GetPodName(), svcName, p.GetPodNamespace()),
		},
	}

	for _, ip := range p.GetPodIPs() {
		addresses = append(addresses, Address{
			IP: net.IP(ip),
		})
	}

	return addresses
}

func (p *PodInfo) GetScopedAddresses(ctx context.Context) ([]Address, error) {
	addresses := []Address{}

	scoped := p.VolumeSelector.Scope

	if scoped.Node == volume.ScopeNode {
		addresses = append(addresses, p.GetNodeIPs(ctx)...)
	}

	if scoped.Pod == volume.ScopePod {
		addresses = append(addresses, p.GetPodAddresses()...)
	}

	if scoped.Services != nil {
		for _, svcName := range scoped.Services {
			addresses = append(addresses, p.GetServiceIPsByName(svcName)...)
		}
	}

	if scoped.ListenerVolumes != nil {
		listenerAddresses, err := p.GetListenerAddresses(ctx)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, listenerAddresses...)
	}

	return addresses, nil
}

// Get listener name, listener name might be empty.
// i.e., no listener volume in scope, or listener name of listener volume does not exist
//
// Compare the listener volume in the scope with the name of the pod's volumes to find a valid listener volume
// Then get the listener name through the annotation of the PVC corresponding to the listener volume
//
// If there is no listener volume in the pod volumes, then return empty, i.e., the pod does not use a listener.
// This situation should be considered normal, because the listener function is optional.
//
// If there is a listener volume in the pod volumes, but the PVC corresponding to the listener volume
// does not have a listener name annotation, then return empty.
func (p *PodInfo) GetListenerNames(ctx context.Context) ([]string, error) {
	volumeAndPvcNames := make(map[string]string) // pod volume name -> pvc name

	for _, volume := range p.Pod.Spec.Volumes {
		if volume.Ephemeral != nil {
			// If the volume is an ephemeral volume, then the volume name is pod name + volume name
			pvcName := fmt.Sprintf("%s-%s", p.GetPodName(), volume.Name)
			logger.V(1).Info("found ephemeral volume in pod", "pod", p.GetPodName(), "namespace", p.GetPodNamespace(), "volume", volume.Name, "pvc", pvcName)
			volumeAndPvcNames[volume.Name] = pvcName
		} else if volume.PersistentVolumeClaim != nil {
			// When the workloads are statefulset, the name is automatically generated by statefulset through pvcTemplate
			// We can directly get the name of the pvc here
			pvcName := volume.PersistentVolumeClaim.ClaimName
			logger.V(1).Info("found pvc volume in pod", "pod", p.GetPodName(), "namespace", p.GetPodNamespace(), "volume", pvcName)
			volumeAndPvcNames[volume.Name] = pvcName
		}
	}

	if len(volumeAndPvcNames) == 0 {
		logger.V(1).Info("can not find any volume in pod, support volume type: PersistentVolumeClaim and ephemeral", "pod", p.GetPodName(), "namespace", p.GetPodNamespace(), "podVolumes", p.Pod.Spec.Volumes)
		return nil, nil
	}

	var listenerNames []string
	for _, listenerVolume := range p.VolumeSelector.Scope.ListenerVolumes {
		listenerPVCName, found := volumeAndPvcNames[listenerVolume]
		if !found {
			logger.V(1).Info("can not find listener volume in pod volumes", "pod", p.GetPodName(),
				"namespace", p.GetPodNamespace(), "listenerVolume", listenerVolume,
			)
			continue
		}

		pvc, err := p.getPVC(ctx, listenerPVCName)
		if err != nil {
			return nil, err
		}

		listenerName, found := pvc.Annotations[listenerUtil.ListenersZncdataListenerName]
		if !found {
			logger.V(1).Info("can not find listener name in listener pvc annotations", "pod", p.GetPodName(),
				"namespace", p.GetPodNamespace(), "listenerVolume", listenerVolume, "listenerPVC", listenerPVCName,
			)
			continue
		}
		listenerNames = append(listenerNames, listenerName)
	}

	if listenerNames == nil {
		logger.V(1).Info("can not find any listener name, because all listener volumes not in pod volumes", "pod", p.GetPodName(),
			"namespace", p.GetPodNamespace(), "listenerVolumes", p.VolumeSelector.Scope.ListenerVolumes, "podVolumes", p.Pod.Spec.Volumes,
		)
		return nil, nil
	}

	return listenerNames, nil
}

func (p *PodInfo) getPVC(ctx context.Context, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := p.client.Get(
		ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: p.GetPodNamespace(),
		},
		pvc,
	)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (p *PodInfo) GetListenerAddresses(ctx context.Context) ([]Address, error) {
	// get listener names from listener volumes, where listener volume is in the scope of the pod
	listenerNames, err := p.GetListenerNames(ctx)
	if err != nil {
		return nil, err
	}

	// If listener name is empty, then return empty
	// This situation should be considered normal, because the listener function is optional.
	if listenerNames == nil {
		return nil, nil
	}

	var addresses []Address

	for _, listenerName := range listenerNames {
		listener, err := p.GetListener(ctx, listenerName)
		if err != nil {
			return nil, err
		}
		for _, ingressAddress := range listener.Status.IngressAddress {
			if ingressAddress.AddressType == listenersv1alpha1.AddressTypeHostname {
				addresses = append(addresses, Address{
					Hostname: ingressAddress.Address,
				})
			} else if ingressAddress.AddressType == listenersv1alpha1.AddressTypeIP {
				addresses = append(addresses, Address{
					IP: net.IP(ingressAddress.Address),
				})
			}
		}
	}

	return addresses, nil
}

func (p *PodInfo) GetListener(ctx context.Context, name string) (*listenersv1alpha1.Listener, error) {
	listener := &listenersv1alpha1.Listener{}

	err := p.client.Get(ctx, client.ObjectKey{Name: name, Namespace: p.GetPodNamespace()}, listener)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

func (p *PodInfo) CheckNodeScope(ctx context.Context, listenerVolume string) (bool, error) {
	scope := p.VolumeSelector.Scope.Node
	if scope == volume.ScopePod {
		return true, nil
	}

	isPodScope, err := p.checkNodeScopeByListener(ctx, listenerVolume)
	if err != nil {
		return false, err
	}

	if isPodScope {
		return true, nil
	}

	return false, nil
}

// Check if the listener's scope is node scope through volume listener.
// Determine whether it is a node port type based on the service type of the listener class.
// First, get the listener class name through the annotation of the listener PVC, if not found,
// get the listener name through the annotation of the PVC, and then get the class name of the corresponding listener spec
func (p *PodInfo) checkNodeScopeByListener(ctx context.Context, listenerVolume string) (bool, error) {
	pvc, err := p.getPVC(ctx, listenerVolume)
	if err != nil {
		return false, err
	}

	// get listener class name from listener pvc annotations
	// if listener class name not found, get it from listener spec
	listenerClassName, found := pvc.Annotations[listenerUtil.ListenersZncdataListenerClass]
	if !found {
		logger.V(1).Info("can not find listener class in listener pvc annotations", "pod", p.GetPodName(), "namespace",
			p.GetPodNamespace(), "listenerVolume", listenerVolume,
		)

		listenerName, found := pvc.Annotations[listenerUtil.ListenersZncdataListenerName]
		if !found {
			logger.V(1).Info("can not find listener name in listener pvc annotations", "pod", p.GetPodName(),
				"namespace", p.GetPodNamespace(), "listenerVolume", listenerVolume,
			)
			return false, nil
		}

		listener, err := p.GetListener(ctx, listenerName)
		if err != nil {
			return false, err
		}

		listenerClassName = listener.Spec.ClassName
	}

	if listenerClassName == "" {
		logger.V(1).Info("can not find listener class name in listener pvc annotations", "pod", p.GetPodName(), "namespace",
			p.GetPodNamespace(), "listenerVolume", listenerVolume,
		)
	}

	listenerClass, err := p.getListenerClass(ctx, listenerClassName)
	if err != nil {
		return false, err
	}

	if listenerClass.Spec.ServiceType == listenersv1alpha1.ServiceTypeNodePort {
		return true, nil
	}

	logger.V(1).Info("listener class service type is not node port", "pod", p.GetPodName(),
		"namespace", p.GetPodNamespace(), "listenerVolume", listenerVolume, "listenerClass",
		listenerClassName, "serviceType", listenerClass.Spec.ServiceType,
	)
	return false, nil
}

func (p *PodInfo) getListenerClass(ctx context.Context, name string) (*listenersv1alpha1.ListenerClass, error) {
	listenerClass := &listenersv1alpha1.ListenerClass{}
	err := p.client.Get(
		ctx,
		client.ObjectKey{
			Name: name,
		},
		listenerClass,
	)
	if err != nil {
		return nil, err
	}

	return listenerClass, nil
}
