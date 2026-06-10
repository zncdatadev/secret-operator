package backend

import (
	"fmt"
	"strings"

	secretsv1alpha1 "github.com/zncdatadev/secret-operator/api/v1alpha1"
	"github.com/zncdatadev/secret-operator/pkg/volume"
)

const (
	// AllowedNamespacesAnnotation is the annotation on SecretClass that specifies
	// a comma-separated list of namespaces allowed for cross-namespace references.
	// If not set, only the Pod's own namespace is allowed.
	AllowedNamespacesAnnotation = "secrets.kubedoop.dev/allowed-namespaces"
)

// blockedNamespaces are system namespaces that are never allowed as cross-namespace
// reference targets, regardless of the SecretClass annotation whitelist.
var blockedNamespaces = map[string]bool{
	"kube-system":    true,
	"kube-public":    true,
	"kube-node-lease": true,
}

// NamespaceValidationError represents a cross-namespace reference policy violation.
type NamespaceValidationError struct {
	PodNamespace     string
	RequestedNamespace string
	SecretClassName    string
	Field             string
}

func (e *NamespaceValidationError) Error() string {
	return fmt.Sprintf(
		"cross-namespace reference denied: SecretClass %q references namespace %q from pod namespace %q (field: %s)",
		e.SecretClassName, e.RequestedNamespace, e.PodNamespace, e.Field,
	)
}

// ValidateCrossNamespaceReferences checks that all namespace references in the SecretClass
// are within the allowed namespaces for the given Pod namespace.
//
// Allowed namespaces are determined by:
//  1. The SecretClass annotation "secrets.kubedoop.dev/allowed-namespaces" (comma-separated list)
//  2. If the annotation is not set, only the Pod's own namespace is allowed
//
// Returns nil if all references are within the allowed set.
func ValidateCrossNamespaceReferences(secretClass *secretsv1alpha1.SecretClass, volumeCtx *volume.SecretVolumeContext) error {
	podNamespace := volumeCtx.PodNamespace
	className := secretClass.Name
	allowed := resolveAllowedNamespaces(secretClass, podNamespace)

	// Remove blocked system namespaces from the allowed set.
	// These can never be cross-namespace reference targets, even with
	// an explicit annotation whitelist.
	for ns := range blockedNamespaces {
		delete(allowed, ns)
	}

	backend := secretClass.Spec.Backend
	if backend == nil {
		return nil
	}

	// Validate Kerberos backend: AdminKeytabSecret.Namespace
	if backend.KerberosKeytab != nil && backend.KerberosKeytab.AdminKeytabSecret != nil {
		ns := backend.KerberosKeytab.AdminKeytabSecret.Namespace
		if !isAllowedNamespace(ns, allowed) {
			return &NamespaceValidationError{
				PodNamespace:      podNamespace,
				RequestedNamespace: ns,
				SecretClassName:   className,
				Field:            "kerberosKeytab.adminKeytabSecret.namespace",
			}
		}
	}


	// Validate AutoTLS backend: CA.Secret.Namespace and AdditionalTrustRoots
	if backend.AutoTls != nil {
		// CA Secret
		if backend.AutoTls.CA != nil && backend.AutoTls.CA.Secret != nil {
			ns := backend.AutoTls.CA.Secret.Namespace
			if !isAllowedNamespace(ns, allowed) {
				return &NamespaceValidationError{
					PodNamespace:      podNamespace,
					RequestedNamespace: ns,
					SecretClassName:   className,
					Field:            "autoTls.ca.secret.namespace",
				}
			}
		}

		// Additional Trust Roots
		for i, root := range backend.AutoTls.AdditionalTrustRoots {
			fieldPrefix := fmt.Sprintf("autoTls.additionalTrustRoots[%d]", i)

			if root.ConfigMap != nil {
				ns := root.ConfigMap.Namespace
				if !isAllowedNamespace(ns, allowed) {
					return &NamespaceValidationError{
						PodNamespace:      podNamespace,
						RequestedNamespace: ns,
						SecretClassName:   className,
						Field:            fieldPrefix + ".configMap.namespace",
					}
				}
			}

			if root.Secret != nil {
				ns := root.Secret.Namespace
				if !isAllowedNamespace(ns, allowed) {
					return &NamespaceValidationError{
						PodNamespace:      podNamespace,
						RequestedNamespace: ns,
						SecretClassName:   className,
						Field:            fieldPrefix + ".secret.namespace",
					}
				}
			}
		}
	}

	// Validate K8sSearch backend: searchNamespace.Name (explicit namespace only)
	// searchNamespace.Pod is safe — it uses the Pod's own namespace implicitly.
	if backend.K8sSearch != nil && backend.K8sSearch.SearchNamespace != nil {
		if backend.K8sSearch.SearchNamespace.Name != nil {
			ns := *backend.K8sSearch.SearchNamespace.Name
			if !isAllowedNamespace(ns, allowed) {
				return &NamespaceValidationError{
					PodNamespace:      podNamespace,
					RequestedNamespace: ns,
					SecretClassName:   className,
					Field:            "k8sSearch.searchNamespace.name",
				}
			}
		}
	}

	return nil
}

// resolveAllowedNamespaces builds the set of allowed namespaces from the SecretClass
// annotation, falling back to only the Pod's own namespace.
func resolveAllowedNamespaces(secretClass *secretsv1alpha1.SecretClass, podNamespace string) map[string]bool {
	allowed := make(map[string]bool)
	// Always allow the Pod's own namespace
	allowed[podNamespace] = true

	annotations := secretClass.GetAnnotations()
	if annotations == nil {
		return allowed
	}

	whitelist, ok := annotations[AllowedNamespacesAnnotation]
	if !ok || whitelist == "" {
		return allowed
	}

	for _, ns := range strings.Split(whitelist, ",") {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			allowed[ns] = true
		}
	}

	return allowed
}

// isAllowedNamespace checks if a namespace is in the allowed set
// and is not in the globally blocked list.
func isAllowedNamespace(namespace string, allowed map[string]bool) bool {
	if blockedNamespaces[namespace] {
		return false
	}
	return allowed[namespace]
}
