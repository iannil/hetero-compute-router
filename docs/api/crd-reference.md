# API 参考文档

**项目**: Hetero-Compute-Router (HCS)
**API Group**: `hcs.io`
**版本**: `v1alpha1`

---

## 概述

HCS API 定义了一组用于异构算力管理的 Kubernetes 自定义资源 (CRD)。

### API 组结构

```
hcs.io/v1alpha1
├── ComputeNode     # 节点算力资源
├── ComputeQuota    # 命名空间算力配额
├── ComputeTask     # 任务算力请求 (可选)
└── ComputeTopology # 集群拓扑信息 (可选)
```

---

## 资源类型

### ComputeNode

描述单个节点的算力资源和设备信息。

#### 规范 (Spec)

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `vendor` | string | 是 | 硬件厂商: `nvidia`, `huawei`, `hygon`, `cambricon` |
| `nodeSelector` | object | 是 | 匹配 Kubernetes Node 的选择器 |
| `capacity.vram` | quantity | 是 | 总显存容量 |
| `capacity.tflops-fp16` | quantity | 是 | FP16 算力总量 (TFLOPS) |
| `capacity.tflops-fp32` | quantity | 是 | FP32 算力总量 (TFLOPS) |
| `devices` | []Device | 是 | 设备列表 |
| `topology` | Topology | 否 | 节点拓扑信息 |

#### Device

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `id` | string | 是 | 设备 ID |
| `model` | string | 是 | 设备型号 |
| `vram` | quantity | 是 | 显存大小 |
| `vramAllocatable` | quantity | 是 | 可分配显存 |
| `vramAllocated` | quantity | 是 | 已分配显存 |
| `compute.fp16` | quantity | 是 | FP16 算力 |
| `compute.fp32` | quantity | 是 | FP32 算力 |
| `topology.busId` | string | 是 | PCI 总线 ID |
| `topology.links` | []Link | 否 | 互联信息 |
| `healthScore` | integer | 是 | 健康评分 (0-100) |

#### Link

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `type` | string | 是 | 互联类型: `nvlink`, `hccs`, `pcie`, `roce` |
| `peers` | []string | 是 | 对端设备 ID 列表 |
| `bandwidth` | string | 是 | 带宽 (如 "600GB/s") |

#### 状态 (Status)

| 字段 | 类型 | 描述 |
|------|------|------|
| `phase` | string | 节点阶段: `Ready`, `NotReady`, `Draining` |
| `conditions` | []Condition | 条件列表 |
| `lastHeartbeat` | time | 最后心跳时间 |

#### 示例

```yaml
apiVersion: hcs.io/v1alpha1
kind: ComputeNode
metadata:
  name: gpu-node-1
  labels:
    vendor: nvidia
    topology.kubernetes.io/zone: zone-a
spec:
  vendor: nvidia
  nodeSelector:
    kubernetes.io/hostname: gpu-node-1
  capacity:
    vram: 640Gi
    tflops-fp16: "2496"
    tflops-fp32: "156"
  devices:
    - id: "0"
      model: A100-80G
      vram: 80Gi
      vramAllocatable: 80Gi
      vramAllocated: 16Gi
      compute:
        fp16: "312"
        fp32: "19.5"
      topology:
        busId: "0000:17:00.0"
        links:
          - type: nvlink
            peers: ["1", "2", "3", "4", "5", "6", "7"]
            bandwidth: "600GB/s"
      healthScore: 95
  topology:
    interconnects:
      - type: nvlink
        bandwidth: "600GB/s"
        nvlinkVersion: "4.0"
status:
  phase: Ready
  conditions:
    - type: Ready
      status: "True"
      reason: NodeReady
      lastTransitionTime: "2026-01-21T10:00:00Z"
    - type: HardwareHealthy
      status: "True"
      reason: AllDevicesHealthy
  lastHeartbeat: "2026-01-21T10:00:00Z"
```

---

### ComputeQuota

定义命名空间级别的算力配额限制。

#### 规范 (Spec)

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `hard` | ResourceList | 是 | 硬限制 |
| `scopes` | []Scope | 否 | 配额作用域 |
| `selector` | LabelSelector | 否 | 选择器 |

#### 状态 (Status)

| 字段 | 类型 | 描述 |
|------|------|------|
| `used` | ResourceList | 已使用量 |
| `hard` | ResourceList | 硬限制 |

#### Scope

| 值 | 描述 |
|------|------|
| `PriorityClass` | 按 PriorityClass 限制 |
| `Terminating` | 终止中的 Pod |
| `NotTerminating` | 非终止中的 Pod |

#### 示例

```yaml
apiVersion: hcs.io/v1alpha1
kind: ComputeQuota
metadata:
  name: team-a-quota
  namespace: team-a
spec:
  hard:
    vram: 2000Gi
    tflops-fp16: "10000"
  scopes:
    - priority: "high"
  selector:
    matchLabels:
      team: team-a
status:
  used:
    vram: 500Gi
    tflops-fp16: "2500"
  hard:
    vram: 2000Gi
    tflops-fp16: "10000"
```

---

## Pod 资源请求

HCS 扩展了 Kubernetes Pod 的资源请求格式。

### 抽象资源类型

| 资源名 | 类型 | 描述 |
|--------|------|------|
| `ai.compute/vram` | quantity | 请求的显存大小 |
| `ai.compute/tflops-fp16` | quantity | 请求的 FP16 算力 |
| `ai.compute/tflops-fp32` | quantity | 请求的 FP32 算力 |
| `ai.compute/devices` | integer | 请求的设备数量 |

### 示例

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: training-pod
spec:
  containers:
    - name: trainer
      image: pytorch:2.0
      resources:
        requests:
          ai.compute/vram: 16Gi
          ai.compute/tflops-fp16: "100"
        limits:
          ai.compute/vram: 32Gi
          ai.compute/tflops-fp16: "200"
```

### 兼容性

HCS 同时支持传统的厂商特定资源请求：

```yaml
# 传统方式 (仍支持)
resources:
  limits:
    nvidia.com/gpu: 2
    huawei.com/ascend: 2

# HCS 方式 (推荐)
resources:
  requests:
    ai.compute/vram: 32Gi
    ai.compute/tflops-fp16: "400"
```

---

## Pod 注解

HCS 使用注解 (Annotations) 来传递调度和运行时信息。

### 调度相关

| 注解键 | 值 | 描述 |
|--------|-----|------|
| `hcs.io/vendor-preference` | `nvidia`, `huawei`, `hygon`, `any` | 厂商偏好 |
| `hcs.io/allow-downgrade` | `true`, `false` | 是否允许降级 |
| `hcs.io/topology-preference` | `high-bandwidth`, `low-latency` | 拓扑偏好 |
| `hcs.io/assigned-vendor` | `nvidia`, `huawei`, `hygon` | (只读) 实际分配的厂商 |
| `hcs.io/assigned-devices` | `0,1,2` | (只读) 实际分配的设备 ID |

### 运行时相关

| 注解键 | 值 | 描述 |
|--------|-----|------|
| `hcs.io/inject-driver` | `true`, `false` | 是否注入驱动 |
| `hcs.io/inject-runtime` | `true`, `false` | 是否注入运行时 |
| `hcs.io/image-rebinding` | `true`, `false` | 是否启用镜像重定 |

### 示例

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: training-pod
  annotations:
    hcs.io/vendor-preference: nvidia
    hcs.io/allow-downgrade: "true"
    hcs.io/topology-preference: high-bandwidth
spec:
  containers:
    - name: trainer
      image: pytorch:2.0
      resources:
        requests:
          ai.compute/vram: 16Gi
```

---

## 调度结果

调度完成后，Pod 会被添加以下注解：

```yaml
metadata:
  annotations:
    hcs.io/assigned-vendor: "nvidia"
    hcs.io/assigned-devices: "0,1"
    hcs.io/assigned-node: "gpu-node-1"
    hcs.io/scheduling-score: "95"
```

---

## Webhook 修改

Admission Webhook 会对 Pod 进行以下修改：

### 1. 卷挂载

```yaml
spec:
  volumes:
    - name: nvidia-driver
      hostPath:
        path: /usr/local/nvidia
    - name: nvidia-libraries
      hostPath:
        path: /run/nvidia/driver
```

### 2. 环境变量

```yaml
spec:
  containers:
    - env:
        - name: NVIDIA_VISIBLE_DEVICES
          value: "0,1"
        - name: LD_LIBRARY_PATH
          value: "/usr/local/nvidia/lib64:$(LD_LIBRARY_PATH)"
        - name: CUDA_VISIBLE_DEVICES
          value: "0,1"
```

### 3. 设备挂载

```yaml
spec:
  containers:
    - resources:
        limits:
          nvidia.com/gpu: 2  # 添加回传统资源用于设备插件
```

---

## 错误状态

### ComputeNode 条件类型

| 条件类型 | 状态 | 描述 |
|----------|------|------|
| `Ready` | True/False | 节点是否就绪 |
| `HardwareHealthy` | True/False | 硬件是否健康 |
| `AgentConnected` | True/False | Agent 是否连接 |
| `Schedulable` | True/False | 是否可调度 |

### 常见错误

| 错误 | 原因 | 解决方法 |
|------|------|----------|
| `InsufficientVRAM` | 节点显存不足 | 调低请求或扩容 |
| `VendorNotSupported` | 厂商不支持 | 检查厂商偏好设置 |
| `AgentNotConnected` | Agent 未连接 | 检查 Agent 状态 |
| `DeviceUnhealthy` | 设备不健康 | 检查硬件或等待恢复 |

---

## API 版本策略

### 版本格式

```
v1alpha1 → v1alpha2 → v1beta1 → v1
  ↑           ↑           ↑         ↑
 初始设计    实验变更    候选版本   稳定版本
```

### 兼容性承诺

- alpha 版本不保证向后兼容
- beta 版本尽量保证向后兼容
- 稳定版本保证向后兼容 (至少 1 年)
