# Hetero-Compute-Router (HCS)

[![CI](https://github.com/iannil/hetero-compute-router/workflows/CI/badge.svg)](https://github.com/iannil/hetero-compute-router/actions)
[![Helm](https://github.com/iannil/hetero-compute-router/workflows/Helm%20Lint/badge.svg)](https://github.com/iannil/hetero-compute-router/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/iannil/hetero-compute-router)](https://goreportcard.com/report/github.com/iannil/hetero-compute-router)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/kubernetes-%3E%3D%201.24-blue)](https://kubernetes.io/)

HCS (Hetero-Compute-Router) is a Kubernetes-native heterogeneous compute virtualization and adaptation layer. It abstracts hardware differences across multiple vendors (NVIDIA, Huawei Ascend, Hygon DCU, Cambricon MLU), enabling AI workloads to achieve "Write Once, Run Anywhere".

[English](README.md) | [中文](README_CN.md)

---

## Table of Contents

- [The Problem](#the-problem)
- [Solution: Software-Hardware Decoupling](#solution-software-hardware-decoupling)
- [Key Features](#key-features)
- [Architecture](#architecture)
- [Supported Hardware](#supported-hardware)
- [Quick Start](#quick-start)
- [Usage Examples](#usage-examples)
- [Configuration](#configuration)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## The Problem

Modern AI infrastructure faces significant challenges:

| Challenge | Description |
| ----------- | ------------- |
| Vendor Lock-in | Applications are tightly coupled to specific hardware (e.g., `nvidia.com/gpu`) |
| Hardware Fragmentation | Diverse GPU/NPU ecosystem (NVIDIA, Huawei, Hygon, Cambricon) with incompatible APIs |
| Coarse-grained Allocation | Entire GPU allocation even when only partial resources are needed |
| Topology Unawareness | Standard schedulers ignore interconnect topology (NVLink, HCCS, PCIe) |
| Poor Fault Tolerance | Lack of sub-health detection and automatic failover for domestic chips |

Traditional Approach:

```
User requests nvidia.com/gpu → Scheduled to NVIDIA node only
```

HCS Approach:

```
User requests ai.compute/vram: 16Gi → HCS analyzes cluster state →
Dynamically allocates NVIDIA A100 OR Huawei Ascend 910B →
Automatically injects corresponding drivers and environment variables
```

---

## Solution: Software-Hardware Decoupling

HCS implements a three-layer decoupling model that separates user workloads from hardware specifics:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           User Request Layer                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Pod Request: ai.compute/vram: 16Gi, ai.compute/tflops-fp16: "100"  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Scheduling & Injection Layer (HCS)                      │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │   Scheduler     │───▶│    Webhook      │───▶│  Modified Pod   │          │
│  │   Extension     │    │    Injector     │    │  (with drivers) │          │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘          │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Resource Abstraction Layer (URA)                        │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      ComputeNode CRD                                │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                 │    │
│  │  │ NVIDIA  │  │ HUAWEI  │  │  HYGON  │  │CAMBRICON│                 │    │
│  │  │ A100x8  │  │ 910Bx8  │  │ DCUx8   │  │ MLUx8   │                 │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Features

### 1. Unified Resource Abstraction (URA)

Node-Agent deployed on every node collects hardware information using native APIs (NVML, DSMI, CNMon) and reports a "Compute Fingerprint":

- VRAM: Total and available video memory
- Compute Capability: FP16/FP32/INT8 TFLOPS
- Interconnect Topology: NVLink/HCCS/RoCE/PCIe bandwidth and latency
- Health Score: Real-time hardware health monitoring (critical for domestic chips)

### 2. Topology-Aware Scheduling

The HCS scheduler extends the Kubernetes Scheduling Framework with intelligent plugins:

| Plugin | Function |
| -------- | ---------- |
| Filter | Eliminates nodes that don't meet compute requirements |
| Score | Prioritizes nodes with high-bandwidth interconnects and optimal fragmentation |
| Reserve | Pre-allocates VRAM quota before binding |
| Bind | Attaches hardware binding annotations |

Scoring Factors:

- Interconnect affinity (prefer NVLink/HCCS over PCIe)
- Bin-packing optimization (minimize fragmentation)
- Health-weighted scoring (deprioritize sub-healthy nodes)
- Compute exchange rates (cross-vendor equivalence)

### 3. Runtime Injection (Mutating Webhook)

The Runtime Injector automatically configures container environments based on target hardware:

```yaml
# Before HCS Webhook
spec:
  containers:
  - name: pytorch
    image: pytorch/pytorch:latest
    resources:
      requests:
        ai.compute/vram: "16Gi"

# After HCS Webhook (auto-injected for Huawei Ascend)
spec:
  containers:
  - name: pytorch
    image: pytorch/pytorch:latest
    env:
    - name: ASCEND_VISIBLE_DEVICES
      value: "0,1"
    - name: LD_LIBRARY_PATH
      value: "/usr/local/Ascend/driver/lib64"
    volumeMounts:
    - name: ascend-driver
      mountPath: /usr/local/Ascend
  volumes:
  - name: ascend-driver
    hostPath:
      path: /usr/local/Ascend
```

### 4. Software-Defined VRAM Slicing

libhcs_interceptor.so implements quota enforcement without hardware virtualization. The library (913 lines of C code) supports CUDA, ACL (Huawei Ascend), and HIP (Hygon/AMD) APIs:

```
┌─────────────────────────────────────────┐
│         Application (PyTorch)           │
└─────────────────┬───────────────────────┘
                  │ cudaMalloc()
                  ▼
┌─────────────────────────────────────────┐
│   libhcs_interceptor.so (LD_PRELOAD)    │
│  - Intercepts cudaMalloc/aclrtMalloc    │
│  - Enforces VRAM quota (HCS_VRAM_QUOTA) │
│  - Returns OOM if quota exceeded        │
└─────────────────┬───────────────────────┘
                  ▼
┌─────────────────────────────────────────┐
│      Vendor Driver (CUDA/Ascend)        │
└─────────────────────────────────────────┘
```

### 5. Compute Exchange Rates

Enables cross-vendor scheduling with performance equivalence:

```yaml
scheduler:
  exchangeRates:
    nvidia-a100: 1.0      # Baseline
    nvidia-a800: 0.95
    nvidia-h100: 1.5
    huawei-910b: 0.85
    hygon-dcu: 0.6
```

---

## Architecture

### Component Overview

| Component | Type | Description |
| ----------- | ------ | ------------- |
| Node-Agent | DaemonSet | Collects hardware info, reports ComputeNode CRD |
| Scheduler | Deployment | Extends K8s scheduler with compute-aware plugins |
| Webhook | Deployment | Mutates Pods with runtime environment injection |
| Interceptor | Library | Software VRAM slicing via LD_PRELOAD (CUDA/ACL/HIP) |
| eBPF Monitor | Module | Sub-health detection framework (gpu/pcie/health events) |

### ComputeNode CRD

```yaml
apiVersion: hetero.zrs.io/v1alpha1
kind: ComputeNode
metadata:
  name: gpu-node-01
spec:
  vendor: nvidia
  devices:
    - id: "0"
      model: "A100-80G"
      vram: "80Gi"
      compute:
        fp16: "312"
        fp32: "19.5"
      topology:
        busId: "0000:17:00.0"
        links:
          - type: nvlink
            peers: ["1", "2", "3"]
            bandwidth: "600GB/s"
      healthScore: 100
status:
  phase: Ready
  vramAllocatable: "80Gi"
  vramAllocated: "16Gi"
```

---

## Supported Hardware

| Vendor | Product | Detection | Scheduling | Injection | VRAM Slicing |
| -------- | --------- | ----------- | ------------ | ----------- | -------------- |
| NVIDIA | A100/A800/H100/V100 | ✅ Complete | ✅ Complete | ✅ Complete | ✅ Implemented |
| Hygon | DCU Z100/Z100L | ✅ Complete | ✅ Complete | ✅ Complete | ✅ Implemented |
| Huawei | Ascend 910A/910B | 🔄 Mock | ✅ Complete | ✅ Complete | ✅ Implemented |
| Cambricon | MLU370 | 🔜 Planned | 🔜 Planned | ✅ Profile Ready | 🔜 Planned |

---

## Quick Start

### Prerequisites

- Kubernetes 1.24+
- Helm 3.10+
- cert-manager 1.12+ (recommended for TLS)
- At least one GPU/NPU node

### Installation

Option 1: OCI Registry (Recommended)

```bash
helm install hcs oci://ghcr.io/iannil/hetero-compute-router/charts/hcs \
  --namespace hcs-system \
  --create-namespace
```

Option 2: From Source

```bash
# Clone repository
git clone https://github.com/iannil/hetero-compute-router.git
cd hetero-compute-router

# Install CRDs
kubectl apply -f config/crd/

# Install with Helm
helm install hcs ./chart/hcs \
  --namespace hcs-system \
  --create-namespace
```

### Verify Installation

```bash
# Check pods
kubectl get pods -n hcs-system

# Expected output:
# NAME                             READY   STATUS    RESTARTS   AGE
# hcs-node-agent-xxxxx             1/1     Running   0          1m
# hcs-scheduler-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
# hcs-webhook-xxxxxxxxxx-xxxxx     1/1     Running   0          1m

# Check ComputeNode resources
kubectl get computenodes

# Expected output (with GPU nodes):
# NAME          VENDOR   NODE        PHASE   VRAM          AGE
# gpu-node-01   nvidia   gpu-node-01 Ready   85899345920   1m
```

---

## Usage Examples

### Basic Usage

Request abstract compute resources instead of vendor-specific ones:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ai-workload
spec:
  schedulerName: hcs-scheduler
  containers:
  - name: pytorch
    image: pytorch/pytorch:latest
    resources:
      requests:
        ai.compute/vram: "16Gi"
        ai.compute/tflops-fp16: "100"
```

### PyTorch Training Example

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pytorch-training
  labels:
    app: pytorch
spec:
  schedulerName: hcs-scheduler
  containers:
  - name: pytorch
    image: pytorch/pytorch:2.1.0-cuda12.1-cudnn8-runtime
    command: ["python", "train.py"]
    resources:
      requests:
        ai.compute/vram: "32Gi"
      limits:
        ai.compute/vram: "64Gi"
```

### Multi-GPU Job with Topology Preference

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: distributed-training
  annotations:
    hcs.io/topology-preference: "high-bandwidth"
spec:
  schedulerName: hcs-scheduler
  containers:
  - name: pytorch
    image: pytorch/pytorch:latest
    resources:
      requests:
        ai.compute/vram: "160Gi"  # 2x 80GB GPUs
        ai.compute/tflops-fp16: "600"
```

---

## Configuration

### Helm Values Overview

| Parameter | Description | Default |
| ----------- | ------------- | --------- |
| `nodeAgent.enabled` | Enable Node-Agent DaemonSet | `true` |
| `nodeAgent.logLevel` | Log level (debug/info/warn/error) | `info` |
| `nodeAgent.reportInterval` | Heartbeat interval in seconds | `30` |
| `scheduler.enabled` | Enable HCS Scheduler | `true` |
| `scheduler.replicas` | Number of scheduler replicas | `1` |
| `scheduler.leaderElection.enabled` | Enable leader election | `true` |
| `webhook.enabled` | Enable Mutating Webhook | `true` |
| `webhook.replicas` | Number of webhook replicas | `2` |
| `webhook.failurePolicy` | Webhook failure policy | `Fail` |

### Environment-Specific Configurations

Development:

```bash
helm install hcs ./chart/hcs \
  -f ./chart/hcs/values-dev.yaml \
  --namespace hcs-dev \
  --create-namespace
```

Production:

```bash
helm install hcs ./chart/hcs \
  -f ./chart/hcs/values-prod.yaml \
  --namespace hcs-system \
  --create-namespace
```

For complete configuration reference, see [Configuration Guide](docs/deployment/configuration.md).

---

## Roadmap

### Phase 1: The Observer (MVP) ✅ Complete

- [x] Node-Agent with NVIDIA/Hygon/Ascend detection
- [x] ComputeNode CRD definition
- [x] Basic scheduling logic (Filter/Score/Reserve plugins)
- [x] Helm Chart deployment
- [x] eBPF monitoring framework
- [x] ~75% unit test coverage

### Phase 2: The Router ✅ Mostly Complete

- [x] Compute exchange rate conversion (15+ hardware profiles)
- [x] Mutating Admission Webhook
- [x] Driver and environment injection (NVIDIA/Huawei/Hygon/Cambricon)
- [x] `libhcs_interceptor.so` implementation (CUDA/ACL/HIP APIs)
- [ ] Cross-vendor compatibility testing on real hardware
- [ ] Cambricon MLU detector implementation

### Phase 3: The Virtualizer (In Progress)

- [x] `libhcs_interceptor.so` for VRAM slicing - ✅ Implemented
- [x] eBPF programs written (gpu_monitor, pcie_monitor, health_events)
- [ ] eBPF program compilation integration
- [ ] Dynamic image rebinding
- [ ] Sub-health node automatic isolation
- [ ] Automatic checkpoint recovery

### Version Timeline

| Version | Target | Features |
| --------- | -------- | ---------- |
| v0.1.0-alpha | Q1 2026 | Phase 1 MVP ✅ |
| v0.2.0-beta | Q2 2026 | Phase 2 Complete (current) |
| v0.3.0-beta | Q3 2026 | Phase 3 Complete |
| v1.0.0 | Q4 2026 | Production Ready |

### Current Metrics

| Metric | Value |
| ------- | ------ |
| Go Code Lines | ~9,500 |
| C Code Lines | ~1,200 (interceptor + eBPF) |
| Test Coverage | ~75% |
| Supported Vendors | 3 (NVIDIA, Hygon, Huawei) |
| eBPF Programs | 3 |

---

## Comparison with Alternatives

| Feature | K8s Native | Volcano/YuniKorn | HCS |
| --------- | ------------ | ------------------ | --------- |
| Resource Granularity | Whole GPU | Static MPS | Dynamic soft-slicing |
| Heterogeneous Support | Label-based | Device Plugin | Unified Abstraction |
| Runtime Environment | Manual | Manual | Auto-injection |
| Topology Awareness | None | NUMA only | Cross-chip interconnect |
| Fault Tolerance | Pod restart | Job retry | Sub-health isolation + checkpoint |
| Positioning | Resource scheduling | Batch scheduling | Compute virtualization layer |

---

## Building from Source

```bash
# Clone repository
git clone https://github.com/iannil/hetero-compute-router.git
cd hetero-compute-router

# Build all binaries
make build

# Run tests
make test

# Build Docker images
make docker-build

# Push images (requires authentication)
make docker-push
```

---

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### Development Setup

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Run linting: `make lint`
5. Submit a pull request

### Code Structure

```
hetero-compute-router/
├── cmd/
│   ├── node-agent/      # Node-Agent entry point
│   ├── scheduler/       # Scheduler extension entry point
│   └── webhook/         # Webhook server entry point
├── pkg/
│   ├── api/v1alpha1/    # CRD types and deepcopy
│   ├── agent/           # Node-Agent logic
│   ├── collectors/      # Hardware collectors
│   ├── detectors/       # Hardware detectors (NVML, DCU, Ascend)
│   │   ├── nvidia/      # NVIDIA NVML detector (complete)
│   │   ├── hygon/       # Hygon DCU detector (complete)
│   │   └── ascend/      # Huawei Ascend detector (mock)
│   ├── exchange/        # Compute exchange rates (15+ profiles)
│   ├── interceptor/     # API hijack library (libhcs_interceptor.so)
│   ├── monitoring/ebpf/ # eBPF health monitoring
│   │   └── programs/    # eBPF C programs (gpu/pcie/health)
│   ├── scheduler/       # Scheduler plugins (Filter/Score/Reserve)
│   └── webhook/         # Admission webhook + HCS injector
├── chart/hcs/           # Helm Chart
├── config/              # Kubernetes manifests
├── docs/                # Documentation
├── test/                # Integration tests
└── hack/                # Build scripts
```

---

## Community

- GitHub Issues: [Report bugs or request features](https://github.com/iannil/hetero-compute-router/issues)
- Discussions: [Ask questions and share ideas](https://github.com/iannil/hetero-compute-router/discussions)

---

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

## Acknowledgments

HCS is inspired by the challenges of managing heterogeneous AI infrastructure in production environments. Special thanks to:

- The Kubernetes Scheduling Framework for extensible scheduling
- NVIDIA NVML and Huawei DSMI for hardware introspection APIs
- The cloud-native community for continuous innovation

---

HCS: Eliminating vendor lock-in, one cluster at a time.
