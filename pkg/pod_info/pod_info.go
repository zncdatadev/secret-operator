package pod_info

import (
	"context"
	"fmt"
	"net"

	operatorlistenersv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/listeners/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/secret-operator/pkg/volume"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("pod-info")
)

type PodInfo struct {
	client client.Client
	Pod    *corev1.Pod
	Scope  *volume.SecretScope
}

func NewPodInfo(
	client client.Client,
	pod *corev1.Pod,
	scope *volume.SecretScope,
) *PodInfo {
	return &PodInfo{
		client: client,
		Pod:    pod,
		Scope:  scope,
	}
}

func (p *PodInfo) getPodName() string {
	return p.Pod.GetName()
}

func (p *PodInfo) getPodNamespace() string {
	return p.Pod.GetNamespace()
}

func (p *PodInfo) getPodIP() string {
	return p.Pod.Status.PodIP
}

// Get the pod's IP address
// k8s assign ips for pod when pvc is successfully bound,
// so it is empty when pvc is not bound
func (p *PodInfo) getPodIPs() []string {
	ips := []string{}
	for _, address := range p.Pod.Status.PodIPs {
		ips = append(ips, address.IP)
	}
	return ips
}

func (p *PodInfo) getNode(ctx context.Context) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := p.client.Get(
		ctx,
		client.ObjectKey{
			Name: p.Pod.Spec.NodeName,
		},
		node,
	)
	if err != nil {
		return nil, err
	}

	return node, nil

}

// Get the address information of the node where the pod is located.
func (p *PodInfo) getNodeAddresses(ctx context.Context) ([]Address, error) {
	node, err := p.getNode(ctx)
	if err != nil {
		return nil, nil
	}

	addresses := []Address{
		{
			Hostname: node.Name,
		},
	}

	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
			ip := net.ParseIP(address.Address)
			if ip == nil {
				return nil, fmt.Errorf("invalid node ip: %s", address.Address)
			}
			addresses = append(addresses, Address{IP: ip})
		}
	}

	logger.V(1).Info("get node ip filter by internal and external", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "addresses", addresses)
	return addresses, nil
}

// TODO: Dynamic get cluster domain, currently hard code to cluster.local
func (p *PodInfo) getFQDN(subdomain string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", subdomain, p.getPodNamespace())
}

func (p *PodInfo) getFQDNAddress(subdomain string) Address {
	return Address{Hostname: p.getFQDN(subdomain)}
}

// Get the address information of the pod.
// In statusfulset, the spec.serviceName field is required, so the pod will come with pod.spec.subdomain.
// In deployment, the pod does not have pod.spec.subdomain by default. If needed, you can first create a Service,
// and then configure the subdomain field for the podTemplate in the deployment.
func (p *PodInfo) getPodAddresses() ([]Address, error) {
	addresses := make([]Address, 0)
	svcName := p.Pod.Spec.Subdomain
	if svcName != "" {
		// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pods
		addresses = append(
			addresses,
			p.getFQDNAddress(svcName),
			p.getFQDNAddress(fmt.Sprintf("%s.%s", p.getPodName(), svcName)),
		)
	}

	for _, ipStr := range p.getPodIPs() {
		ip := net.ParseIP(ipStr)

		if ip == nil {
			return nil, fmt.Errorf("invalid pod ip: %s from pod %s", ipStr, p.getPodName())
		}
		addresses = append(addresses, Address{IP: ip})
	}

	logger.V(1).Info("get pod addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "addresses", addresses)
	return addresses, nil
}

func (p *PodInfo) getServiceAddresses(serviceNames []string) []Address {
	addresses := make([]Address, 0)
	for _, svcName := range serviceNames {
		addresses = append(addresses, p.getFQDNAddress(svcName))
	}
	logger.V(1).Info("get service addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "services", serviceNames, "addresses", addresses)
	return addresses
}

// Get the address information of the pod according to the scope.
func (p *PodInfo) GetScopedAddresses(ctx context.Context) ([]Address, error) {
	addresses := make([]Address, 0)

	scoped := p.Scope

	if scoped.Node == volume.ScopeNode {
		nodeAddresses, err := p.getNodeAddresses(ctx)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, nodeAddresses...)
	}

	if scoped.Pod == volume.ScopePod {
		podAddresses, err := p.getPodAddresses()
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, podAddresses...)
	}

	if scoped.Services != nil {
		serviceAddresses := p.getServiceAddresses(scoped.Services)
		addresses = append(addresses, serviceAddresses...)
	}

	if scoped.ListenerVolumes != nil {
		listenerAddresses, err := p.getListenerAddresses(ctx)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, listenerAddresses...)
	}

	logger.V(1).Info("get scoped addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "scope", scoped, "addresses", addresses)
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
func (p *PodInfo) getListenerNames(ctx context.Context) ([]string, error) {
	volumeNameToPvcName := make(map[string]string) // pod volume name -> pvc name

	for _, volume := range p.Pod.Spec.Volumes {
		if volume.Ephemeral != nil {
			// If the volume is an ephemeral volume, then the generated pvc name format is `<podName>-<volumeName>`
			// Whether it's a standalone pod, statefulset, or deployment, the ephemeral volume's pvc name format is the same.
			pvcName := fmt.Sprintf("%s-%s", p.getPodName(), volume.Name)
			logger.V(5).Info("found ephemeral volume in pod", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "volume", volume.Name, "pvc", pvcName)
			volumeNameToPvcName[volume.Name] = pvcName
		} else if volume.PersistentVolumeClaim != nil {
			// When the workloads are statefulset, the name is automatically generated by statefulset through pvcTemplate
			// We can directly get the name of the pvc here.
			// Or when the workloads (statefulset or deployment), the name is specified by the user through the pvc field.
			pvcName := volume.PersistentVolumeClaim.ClaimName
			logger.V(5).Info("found pvc volume in pod", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "volume", pvcName)
			volumeNameToPvcName[volume.Name] = pvcName
		}
	}

	if len(volumeNameToPvcName) == 0 {
		logger.V(1).Info("can not find any volume in pod, support volume type: PersistentVolumeClaim and ephemeral",
			"pod", p.getPodName(), "namespace", p.getPodNamespace(), "podVolumes", p.Pod.Spec.Volumes,
		)
		return make([]string, 0, 0), nil
	}

	// Filter listener volumes in the scope
	// Get the listener name through the annotation of the PVC corresponding to the listener volume
	listenerNames := make([]string, 0)
	for _, listenerVolume := range p.Scope.ListenerVolumes {
		listenerPVCName, found := volumeNameToPvcName[listenerVolume]
		if !found {
			logger.V(1).Info("can not find listener volume in pod volumes", "pod", p.getPodName(),
				"namespace", p.getPodNamespace(), "listenerVolume", listenerVolume,
			)
			continue
		}

		pvc, err := p.getPVC(ctx, listenerPVCName)
		if err != nil {
			return nil, err
		}

		listenerName, found := pvc.Annotations[constants.AnnotationListenerName]
		if !found {
			logger.V(1).Info("can not find listener name in listener pvc annotations", "pod", p.getPodName(),
				"namespace", p.getPodNamespace(), "listenerVolume", listenerVolume, "listenerPVC", listenerPVCName,
			)
			continue
		}
		listenerNames = append(listenerNames, listenerName)
	}

	if listenerNames == nil {
		logger.V(1).Info("can not find any listener name, because all listener volumes not in pod volumes",
			"pod", p.getPodName(), "namespace", p.getPodNamespace(), "listenerVolumes", p.Scope.ListenerVolumes,
			"podVolumes", p.Pod.Spec.Volumes,
		)
		return nil, nil
	}

	logger.V(1).Info("get listener names", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
		"listenerNames", listenerNames)

	return listenerNames, nil
}

func (p *PodInfo) getPVC(ctx context.Context, name string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := p.client.Get(
		ctx,
		client.ObjectKey{
			Name:      name,
			Namespace: p.getPodNamespace(),
		},
		pvc,
	)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

// Get listener addresses, listener address might be empty.
func (p *PodInfo) getListenerAddresses(ctx context.Context) ([]Address, error) {
	// get listener names from listener volumes, where listener volume is in the scope of the pod
	listenerNames, err := p.getListenerNames(ctx)
	if err != nil {
		return nil, err
	}

	// If listener name is empty, then return empty
	// This situation should be considered normal, because the listener function is optional.
	if listenerNames == nil {
		logger.V(1).Info("can not find any listener name, this may be normal, because the listener function is optional")
		return nil, nil
	}

	var addresses []Address

	for _, listenerName := range listenerNames {
		listener, err := p.getListener(ctx, listenerName)
		if err != nil {
			return nil, err
		}
		for _, ingressAddress := range listener.Status.IngressAddresses {
			if ingressAddress.AddressType == operatorlistenersv1alpha1.AddressTypeHostname {
				addresses = append(addresses, Address{
					Hostname: ingressAddress.Address,
				})
				logger.V(1).Info("get listener address", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
					"listenerName", listenerName, "address", ingressAddress.Address)
			} else if ingressAddress.AddressType == operatorlistenersv1alpha1.AddressTypeIP {
				ip := net.ParseIP(ingressAddress.Address)
				if ip == nil {
					return nil, fmt.Errorf("invalid listener ip: %s from listener %s", ingressAddress.Address, listenerName)
				}
				addresses = append(addresses, Address{
					IP: ip,
				})
				logger.V(1).Info("get listener address", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
					"listenerName", listenerName, "address", ingressAddress.Address)
			}
		}
	}

	logger.V(1).Info("get listener addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(), "addresses", addresses)
	return addresses, nil
}

func (p *PodInfo) getListener(ctx context.Context, name string) (*operatorlistenersv1alpha1.Listener, error) {
	listener := &operatorlistenersv1alpha1.Listener{}

	err := p.client.Get(ctx, client.ObjectKey{Name: name, Namespace: p.getPodNamespace()}, listener)
	if err != nil {
		return nil, err
	}

	return listener, nil
}

// TODO: in k8sSearch impl, we need to check if the listener is node scope to determine whether to
// search with node scope label
func (p *PodInfo) CheckNodeScope(ctx context.Context, listenerVolume string) (bool, error) {
	scope := p.Scope.Node
	if scope == volume.ScopeNode {
		return true, nil
	}

	isNodeScope, err := p.checkNodeScopeByListener(ctx, listenerVolume)
	if err != nil {
		return false, err
	}

	if isNodeScope {
		return true, nil
	}

	return false, nil
}

// Check if the listener's scope is node scope through volume listener.
// Determine whether it is a node port type based on the service type of the listener class.
// First, get the listener class name through the annotation of the listener PVC, if not found,
// get the listener name through the annotation of the PVC, and then get the class name of the corresponding
// listener spec
func (p *PodInfo) checkNodeScopeByListener(ctx context.Context, listenerVolume string) (bool, error) {
	pvc, err := p.getPVC(ctx, listenerVolume)
	if err != nil {
		return false, err
	}

	// get listener class name from listener pvc annotations
	// if listener class name not found, get it from listener spec
	listenerClassName, found := pvc.Annotations[constants.AnnotationListenersClass]
	if !found {
		logger.V(1).Info("can not find listener class in listener pvc annotations", "pod", p.getPodName(), "namespace",
			p.getPodNamespace(), "listenerVolume", listenerVolume,
		)

		listenerName, found := pvc.Annotations[constants.AnnotationListenerName]
		if !found {
			logger.V(1).Info("can not find listener name in listener pvc annotations", "pod", p.getPodName(),
				"namespace", p.getPodNamespace(), "listenerVolume", listenerVolume,
			)
			return false, nil
		}

		listener, err := p.getListener(ctx, listenerName)
		if err != nil {
			return false, err
		}

		listenerClassName = listener.Spec.ClassName
	}

	if listenerClassName == "" {
		logger.V(1).Info("can not find listener class name in listener pvc annotations", "pod", p.getPodName(), "namespace",
			p.getPodNamespace(), "listenerVolume", listenerVolume,
		)
	}

	listenerClass, err := p.getListenerClass(ctx, listenerClassName)
	if err != nil {
		return false, err
	}

	if *listenerClass.Spec.ServiceType == corev1.ServiceTypeNodePort {
		return true, nil
	}

	logger.V(1).Info("listener class service type is not node port", "pod", p.getPodName(),
		"namespace", p.getPodNamespace(), "listenerVolume", listenerVolume, "listenerClass",
		listenerClassName, "serviceType", listenerClass.Spec.ServiceType,
	)
	return false, nil
}

func (p *PodInfo) getListenerClass(ctx context.Context, name string) (*operatorlistenersv1alpha1.ListenerClass, error) {
	listenerClass := &operatorlistenersv1alpha1.ListenerClass{}
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
