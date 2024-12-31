package pod_info

import (
	"context"
	"fmt"
	"net"
	"sync"

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

	listenerVolumesToListenerCache map[string]string
	mut                            sync.RWMutex
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

// Get the address information of the node where the pod is located.
func (p *PodInfo) getNodeAddresses(ctx context.Context) ([]Address, error) {
	node := &corev1.Node{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: p.Pod.Spec.NodeName}, node); err != nil {
		return nil, nil
	}

	addresses := []Address{{Hostname: node.Name}}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP || address.Type == corev1.NodeExternalIP {
			ip := net.ParseIP(address.Address)
			if ip == nil {
				return nil, fmt.Errorf("invalid node ip: %s", address.Address)
			}
			addresses = append(addresses, Address{IP: ip})
		}
	}

	logger.V(1).Info("got node ip filter by internal and external", "pod", p.getPodName(),
		"namespace", p.getPodNamespace(), "addresses", addresses)
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
	logger.V(1).Info("get service addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
		"services", serviceNames, "addresses", addresses)
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

	logger.V(1).Info("get scoped addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
		"scope", scoped, "addresses", addresses)
	return addresses, nil
}

// Get listener volume names to listener name mapping
// Use cache to avoid multiple calls to the API server
func (p *PodInfo) GetListenerVolumeNamesToListenerName(ctx context.Context) (map[string]string, error) {
	// try read cache
	p.mut.RLock()
	if p.listenerVolumesToListenerCache != nil {
		defer p.mut.RUnlock()
		return p.listenerVolumesToListenerCache, nil
	}
	p.mut.RUnlock()

	// cache miss, relock and write cache
	p.mut.Lock()
	defer p.mut.Unlock()

	// check again in case
	if p.listenerVolumesToListenerCache != nil {
		return p.listenerVolumesToListenerCache, nil
	}

	// fetch listener volume names to listener name
	listenerVolumesToListenerName, err := p.fetchListenerVolumeNamesToListenerName(ctx)
	if err != nil {
		return nil, err
	}

	// update cache
	p.listenerVolumesToListenerCache = listenerVolumesToListenerName
	return listenerVolumesToListenerName, nil
}

func (p *PodInfo) fetchListenerVolumeNamesToListenerName(ctx context.Context) (map[string]string, error) {
	listenerVolumeNameToPvcName := p.getListenerVolumesToPVCNameMapping()
	if len(listenerVolumeNameToPvcName) == 0 {
		logger.V(1).Info("no valid listener volumes found in pod",
			"pod", p.getPodName(),
			"namespace", p.getPodNamespace(),
			"listenerVolumes", p.Scope.ListenerVolumes)
		return nil, nil
	}

	listenerVolumesToListenerName := make(map[string]string)
	for listenerVolumeName, pvcName := range listenerVolumeNameToPvcName {
		pvc, err := p.getPVC(ctx, pvcName)
		if err != nil {
			return nil, fmt.Errorf("failed to get PVC %s: %w", pvcName, err)
		}

		if listenerName, exists := pvc.Annotations[constants.AnnotationListenerName]; exists {
			logger.V(5).Info("using listener name from annotation",
				"listenerName", listenerName,
				"pvc", pvcName,
				"namespace", pvc.Namespace)
			listenerVolumesToListenerName[listenerVolumeName] = listenerName
		} else {
			logger.V(5).Info("using PVC name as listener name",
				"pvc", pvcName,
				"namespace", pvc.Namespace)
			listenerVolumesToListenerName[listenerVolumeName] = pvcName
		}
	}

	if len(listenerVolumesToListenerName) == 0 {
		logger.V(1).Info("no listener names found",
			"pod", p.getPodName(),
			"namespace", p.getPodNamespace(),
			"listenerVolumes", p.Scope.ListenerVolumes)
		return nil, nil
	}

	logger.V(1).Info("found listener names", "pod", p.getPodName(),
		"namespace", p.getPodNamespace(), "listenerVolumeToListenerName", listenerVolumesToListenerName)
	return listenerVolumesToListenerName, nil
}

// getListenerVolumesToPVCMapping returns a map of listener volume name to PVC name
// filter out listener volumes that are not in the secret volume scope of the pod
func (p *PodInfo) getListenerVolumesToPVCNameMapping() map[string]string {
	volumeNameToPvcName := p.getPodVolumeNameToPVCNameMapping()
	if len(volumeNameToPvcName) == 0 {
		logger.V(1).Info("no valid volumes found in pod",
			"pod", p.getPodName(),
			"namespace", p.getPodNamespace())
		return nil
	}

	filteredMapping := make(map[string]string)
	for _, listenerVolume := range p.Scope.ListenerVolumes {
		if pvcName, exists := volumeNameToPvcName[listenerVolume]; exists {
			filteredMapping[listenerVolume] = pvcName
		} else {
			logger.V(1).Info("listener volume not found in pod volumes",
				"pod", p.getPodName(),
				"namespace", p.getPodNamespace(),
				"listenerVolume", listenerVolume)
		}
	}
	return filteredMapping
}

// getPodVolumeNameToPVCMapping returns a map of volume name to PVC name
func (p *PodInfo) getPodVolumeNameToPVCNameMapping() map[string]string {
	volumeNameToPvcName := make(map[string]string)
	for _, volume := range p.Pod.Spec.Volumes {
		if volume.Ephemeral != nil {
			pvcName := fmt.Sprintf("%s-%s", p.getPodName(), volume.Name)
			logger.V(5).Info("found ephemeral volume",
				"pod", p.getPodName(),
				"namespace", p.getPodNamespace(),
				"volume", volume.Name,
				"pvc", pvcName)
			volumeNameToPvcName[volume.Name] = pvcName
		} else if volume.PersistentVolumeClaim != nil {
			pvcName := volume.PersistentVolumeClaim.ClaimName
			logger.V(5).Info("found pvc volume",
				"pod", p.getPodName(),
				"namespace", p.getPodNamespace(),
				"volume", volume.Name,
				"pvc", pvcName)
			volumeNameToPvcName[volume.Name] = pvcName
		}
	}

	return volumeNameToPvcName
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
	listenerVolumesToListenerName, err := p.GetListenerVolumeNamesToListenerName(ctx)
	if err != nil {
		return nil, err
	}

	// If listener name is empty, then return empty
	// This situation should be considered normal, because the listener function is optional.
	if len(listenerVolumesToListenerName) == 0 {
		logger.V(1).Info("can not find any listener name, this may be normal, because the listener function is optional")
		return nil, nil
	}

	addresses := make([]Address, 0)
	for _, listenerName := range listenerVolumesToListenerName {
		listener, err := p.getListener(ctx, listenerName)
		if err != nil {
			return nil, err
		}

		// check listener status
		if len(listener.Status.IngressAddresses) == 0 {
			return nil, fmt.Errorf("listener %s/%s status not ready", listener.Namespace, listener.Name)
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
				addresses = append(addresses, Address{IP: ip})
				logger.V(1).Info("get listener address", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
					"listenerName", listenerName, "address", ingressAddress.Address)
			}
		}
	}

	logger.V(1).Info("get listener addresses", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
		"addresses", addresses)
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

// Check secret listener volume scope has node scope
func (p *PodInfo) HasListenerNodeScope(ctx context.Context) (bool, error) {
	listenerVolumesToListenerName, err := p.GetListenerVolumeNamesToListenerName(ctx)
	if err != nil {
		return false, err
	}

	if len(listenerVolumesToListenerName) == 0 {
		logger.V(1).Info("no listener volumes found in pod", "pod", p.getPodName(), "namespace", p.getPodNamespace())
		return false, nil
	}

	for _, listenerName := range listenerVolumesToListenerName {
		if hasNodeScope, err := p.checkNodeScopeByListener(ctx, listenerName); err != nil {
			logger.V(1).Info("listener volume is not node scope", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
				"listenerVolume", listenerName)
			return false, err
		} else if hasNodeScope {
			logger.V(1).Info("listener volume is node scope", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
				"listenerVolume", listenerName)
			return true, nil
		}
	}
	return false, nil
}

// Check if the listener's scope is node scope through the listener
func (p *PodInfo) checkNodeScopeByListener(ctx context.Context, listenerName string) (bool, error) {
	listener, err := p.getListener(ctx, listenerName)
	if err != nil {
		return false, err
	}

	listenerClass, err := p.getListenerClass(ctx, listener.Spec.ClassName)
	if err != nil {
		return false, err
	}

	logger.V(1).Info("check listener class service type", "pod", p.getPodName(), "namespace", p.getPodNamespace(),
		"listenerName", listenerName, "listenerClass", listenerClass.Name, "serviceType", *listenerClass.Spec.ServiceType)
	return *listenerClass.Spec.ServiceType == corev1.ServiceTypeNodePort, nil
}

func (p *PodInfo) getListenerClass(ctx context.Context, name string) (*operatorlistenersv1alpha1.ListenerClass, error) {
	listenerClass := &operatorlistenersv1alpha1.ListenerClass{}
	err := p.client.Get(ctx, client.ObjectKey{Name: name}, listenerClass)
	if err != nil {
		return nil, err
	}

	return listenerClass, nil
}
