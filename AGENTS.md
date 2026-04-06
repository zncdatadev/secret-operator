<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-04-06 | Updated: 2026-04-06 -->

# secret-operator

## Purpose
Manages Kubernetes secret lifecycle and credential provisioning for the Kubedoop operator ecosystem. Implements a CSI driver plugin to securely mount secrets into pods, supporting TLS certificates, Kerberos keytabs, and other credential types. Handles creation, rotation, and secure delivery of secrets used by other operators.

## Key Files
| File | Description |
|------|-------------|
| `go.mod` | Go module dependencies |
| `Makefile` | Build and development commands |
| `PROJECT` | Kubebuilder project metadata |
| `build/Dockerfile` | Operator manager container image |
| `build/csiplugin.Dockerfile` | CSI plugin container image |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `api/v1alpha1/` | Kubernetes CRD definitions for secret classes and bindings |
| `cmd/` | Operator entry point (`main.go`) and CSI plugin entry point (`csiplugin/`) |
| `config/` | Kubernetes manifests and kustomize configs |
| `internal/controller/` | Controller and reconciliation logic |
| `internal/csi/` | CSI driver implementation (identity, controller, node servers, backends) |
| `internal/util/` | Internal utility helpers |
| `pkg/` | Shared packages |
| `deploy/` | Deployment manifests |
| `test/` | E2E test suites |

## For AI Agents

### Working In This Directory
- Standard Kubebuilder operator structure with an additional CSI plugin component
- Uses `operator-go` framework for reconciliation
- Two binaries: operator manager (`cmd/main.go`) and CSI plugin (`cmd/csiplugin/`)
- Two container images: `build/Dockerfile` (manager) and `build/csiplugin.Dockerfile` (CSI plugin)
- Run `make test` for unit tests
- Run `make deploy` to deploy to cluster

### Testing Requirements
- E2E tests in `test/e2e/`
- Requires Kubernetes cluster for testing
- CSI driver requires node-level access (DaemonSet deployment)

### Common Patterns
- Controllers in `internal/controller/`
- CSI backends in `internal/csi/backend/`
- CRDs use `v1alpha1` API version
- Follows `operator-go` GenericReconciler pattern
- CSI driver implements Container Storage Interface spec for secret injection

## Dependencies

### Internal
- `../operator-go` - Shared operator framework

### External
- `controller-runtime`
- `Kubernetes client-go`
- `github.com/container-storage-interface/spec` - CSI spec
- `github.com/kubernetes-csi/csi-lib-utils` - CSI utilities
- `software.sslmate.com/src/go-pkcs12` - PKCS#12 certificate handling

<!-- MANUAL: -->
