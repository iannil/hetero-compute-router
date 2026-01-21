# 技术架构设计

**项目**: Hetero-Compute-Router (HCS)
**版本**: v0.1.0-design
**更新时间**: 2026-01-21

---

## 1. 总体架构

### 1.1 三层解耦模型

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              用户请求层                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Pod Request: ai.compute/vram: 16Gi, ai.compute/tflops-fp16: "100" │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            调度与注入层 (Layer 2 & 3)                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐          │
│  │  Scheduler      │───▶│   Webhook       │───▶│  Modified Pod   │          │
│  │  Extension      │    │   Injector      │    │  (with drivers) │          │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘          │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            资源抽象层 (Layer 1)                               │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      ComputeNode CRD                                │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                │    │
│  │  │ Node-1  │  │ Node-2  │  │ Node-3  │  │ Node-4  │                │    │
│  │  │ NVIDIA  │  │ HUAWEI  │  │ NVIDIA  │  │ HYGON   │                │    │
│  │  │ A100x8  │  │ 910Bx8  │  │ A800x8  │  │ DCUx8   │                │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘                │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    ▲                                         │
│                                    │                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                       Node-Agent (每节点)                            │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                │    │
│  │  │   NVML  │  │  DSMI   │  │  CNMon  │  │  eBPF   │                │    │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘                │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              硬件层                                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐                        │
│  │ NVIDIA  │  │ HUAWEI  │  │  HYGON  │  │CAMBRICON│                        │
│  │   GPU   │  │   NPU   │  │   DCU   │  │   MLU   │                        │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 组件交互时序

```
User                API Server           Scheduler           Webhook         Node-Agent
 │                      │                    │                   │                  │
 │─ Pod Create ────────▶│                    │                   │                  │
 │                      │─ List ────────────▶│                   │                  │
 │                      │                    │─ Watch ──────────▶│                  │
 │                      │                    │                   │                  │
 │                      │                    │◀── ComputeNode ───│─ Hardware ───────▶│
 │                      │                    │     Status                          │
 │                      │                    │                   │                  │
 │                      │◀── Suggested ──────│                   │                  │
 │      Binding ───────▶│                    │                   │                  │
 │                      │─ Admission Review ───────────────────▶│                  │
 │                      │                    │                   │                  │
 │                      │◀── Patch Pod ─────────────────────────│                  │
 │                      │                    │                   │                  │
 │                      │─ Schedule ────────────────────────────────────────────▶│
 │                      │                    │                   │                  │
 │                      │                    │                   │                  │
 │─ Running Pod ◀──────│                    │                   │                  │
```

---

## 2. 核心组件设计

### 2.1 Node-Agent

**职责**: 节点级硬件监控与资源上报

```go
type NodeAgent struct {
    // 硬件检测器
    detectors map[string]Detector
    // 指标采集器
    collectors map[string]Collector
    // 上报客户端
    client     kubernetes.Interface
}

type Detector interface {
    // 检测硬件类型
    Detect() HardwareType
    // 获取设备列表
    GetDevices() []Device
}

type Collector interface {
    // 采集算力指纹
    CollectFingerprint() Fingerprint
    // 采集健康状态
    CollectHealth() HealthStatus
}
```

**关键模块**:

| 模块 | 功能 | 技术栈 |
|------|------|--------|
| NVML Detector | NVIDIA GPU 检测 | Go-CGO + NVML |
| DSMI Detector | 华为昇腾检测 | Go-CGO + DSMI |
| Topology Collector | 互联拓扑采集 | sysfs + NVML |
| Health Collector | 健康状态采集 | eBPF |
| CRD Reporter | 资源上报 | client-go |

### 2.2 Scheduler Extension

**职责**: 基于算力需求进行智能调度

```go
type HCSPlugin struct {
    handle framework.Handle
    client kubernetes.Interface
}

// Filter: 过滤不符合算力需求的节点
func (p *HCSPlugin) Filter(ctx, state *CycleState, pod *v1.Pod, nodeInfo *NodeInfo) *Status

// Score: 根据拓扑和碎片化程度打分
func (p *HCSPlugin) Score(ctx, state *CycleState, pod *v1.Pod, nodeName string) (int64, *Status)

// Reserve: 预留算力资源
func (p *HCSPlugin) Reserve(ctx, state *CycleState, pod *v1.Pod, nodeName string) *Status

// Bind: 绑定 Pod 到节点
func (p *HCSPlugin) Bind(ctx, state *CycleState, pod *v1.Pod, nodeName string) *Status
```

**调度算法**:

```
1. Filter 阶段:
   - 解析 Pod 算力需求
   - 过滤算力不足的节点
   - 过滤不支持的硬件类型

2. Score 阶段:
   - 拓扑亲和性得分 (高带宽互联优先)
   - 碎片化得分 (优先填满部分空闲节点)
   - 健康状态得分 (亚健康节点降权)

3. Reserve 阶段:
   - 预留显存配额
   - 更新 CRD 资源分配

4. Bind 阶段:
   - 添加硬件绑定注解
```

### 2.3 Runtime Injector Webhook

**职责**: 运行时环境自动注入

```go
type Webhook struct {
    client    kubernetes.Interface
    scheduler SchedulerClient
}

func (w *Webhook) Handle(ctx context.Context, req AdmissionRequest) AdmissionResponse {
    pod := decodePod(req.Object)

    // 1. 获取调度目标
    target := w.scheduler.GetTarget(pod)

    // 2. 根据目标硬件类型注入
    switch target.Vendor {
    case "nvidia":
        return w.injectNvidia(pod, target)
    case "huawei":
        return w.injectHuawei(pod, target)
    // ...
    }
}
```

**注入策略**:

| 类型 | 注入内容 | 示例 |
|------|----------|------|
| 驱动挂载 | HostPath 卷 | `/usr/local/nvidia:/usr/local/nvidia` |
| 环境变量 | LD_LIBRARY_PATH | `/usr/local/nvidia/lib64` |
| 设备选择 | 厂商变量 | `NVIDIA_VISIBLE_DEVICES=0,1` |
| 框架配置 | 通信后端 | `NCCL_P2P_DISABLE=1` |

### 2.4 API Interceptor

**职责**: 软件定义的显存切分

```c
// libhcs_interceptor.c

// 全局状态
typedef struct {
    size_t quota;          // 配额上限
    size_t used;           // 已用量
    bool enforce;          // 是否强制执行
} quota_context_t;

// cudaMalloc wrapper
cudaError_t cudaMalloc(void **ptr, size_t size) {
    if (global_context.used + size > global_context.quota) {
        return cudaErrorMemoryAllocation;
    }

    cudaError_t result = real_cudaMalloc(ptr, size);
    if (result == cudaSuccess) {
        global_context.used += size;
    }
    return result;
}

// 初始化 (通过 LD_PRELOAD 自动执行)
__attribute__((constructor))
void init(void) {
    // 从环境变量读取配额
    char *quota_str = getenv("HCS_VRAM_QUOTA");
    global_context.quota = atol(quota_str);

    // 获取真实函数指针
    real_cudaMalloc = dlsym(RTLD_NEXT, "cudaMalloc");
}
```

---

## 3. CRD 设计

### 3.1 ComputeNode

```yaml
apiVersion: hcs.io/v1alpha1
kind: ComputeNode
metadata:
  name: gpu-node-1
  labels:
    vendor: nvidia
    topology-zone: rack-a
spec:
  vendor: nvidia
  nodeSelector:
    kubernetes.io/hostname: gpu-node-1

  # 算力容量
  capacity:
    vram: "640Gi"        # 8 * 80Gi
    tflops-fp16: "2496"  # 8 * 312 TFLOPS
    tflops-fp32: "156"   # 8 * 19.5 TFLOPS

  # 设备列表
  devices:
    - id: "0"
      model: "A100-80G"
      vram: "80Gi"
      vramAllocatable: "80Gi"
      vramAllocated: "0Gi"
      compute:
        fp16: "312"
        fp32: "19.5"
      topology:
        busId: "0000:17:00.0"
        links:
          - type: nvlink
            peers: ["1", "2", "3", "4", "5", "6", "7"]
            bandwidth: "600GB/s"
      healthScore: 100

  # 互联拓扑
  topology:
    interconnects:
      - type: nvlink
        bandwidth: "600GB/s"
        nvlinkVersion: "4.0"

  # 状态
  status:
    phase: Ready
    conditions:
      - type: Ready
        status: "True"
      - type: HardwareHealthy
        status: "True"
    lastHeartbeat: "2026-01-21T10:00:00Z"
```

### 3.2 ComputeQuota

```yaml
apiVersion: hcs.io/v1alpha1
kind: ComputeQuota
metadata:
  name: team-a-quota
  namespace: team-a
spec:
  hard:
    vram: "2000Gi"
    tflops-fp16: "10000"
  scopes:
    - priority: "high"
  selector:
    matchLabels:
      team: team-a
status:
  used:
    vram: "500Gi"
    tflops-fp16: "2500"
```

---

## 4. 算力指纹模型

### 4.1 指纹结构

```go
type ComputeFingerprint struct {
    // 基本信息
    Vendor    string  // nvidia, huawei, hygon, cambricon
    Model     string  // A100-80G, Ascend910B, ...

    // 显存信息
    VRAMTotal    uint64  // bytes
    VRAMFree     uint64
    VRAMBandwidth uint64 // GB/s

    // 计算能力
    TFP16Flops  uint64  // TFLOPS
    TFP32Flops  uint64
    TensorCores uint16

    // 互联拓扑
    Topology TopologyInfo

    // 健康状态
    HealthScore uint8  // 0-100
}

type TopologyInfo struct {
    NUMANode       int
    PCIEBusID      string
    Interconnects  []Interconnect
    Peers          []PeerInfo
}

type Interconnect struct {
    Type      string  // nvlink, hccs, pcie, roce
    Bandwidth uint64  // GB/s
    Latency   uint64  // ns
}
```

### 4.2 算力汇率

```go
// 算力汇率表 (可配置)
var exchangeRates = map[string]float64{
    "nvidia-a100-80g":  1.0,
    "nvidia-a800-80g":  0.7,
    "huawei-ascend910b": 0.6,
    "huawei-ascend910a": 0.5,
    "hygon-dcu-z100":    0.4,
    "cambricon-mlu370":  0.35,
}

// 计算等效数量
func ComputeEquivalent(requested string, target string) float64 {
    return exchangeRates[requested] / exchangeRates[target]
}
```

---

## 5. 数据流

### 5.1 资源上报流

```
Hardware ──▶ Detector ──▶ Collector ──▶ Aggregator ──▶ CRD Updater ──▶ API Server
            │            │             │              │                │
         NVML/        Fingerprint    合并多设备      生成            更新
         DSMI         计算            数据           CRD             ComputeNode
```

### 5.2 Pod 调度流

```
Pod Create ──▶ Filter ──▶ Score ──▶ Reserve ──▶ Bind ──▶ Webhook ──▶ Injected Pod
                  │         │          │         │           │
               过滤       拓扑       预留      绑定        注入
               节点       打分       配额      注解        驱动
```

---

## 6. 部署架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                               Kubernetes Cluster                             │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                        Control Plane                                │     │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                │     │
│  │  │ API Server  │  │   Scheduler │  │   Webhook   │                │     │
│  │  │  + CRDs     │  │  + Plugin   │  │  Injector   │                │     │
│  │  └─────────────┘  └─────────────┘  └─────────────┘                │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                        │                                     │
│  ┌────────────────────────────────────────────────────────────────────┐     │
│  │                         Worker Nodes                               │     │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │     │
│  │  │   Node-1     │  │   Node-2     │  │   Node-3     │             │     │
│  │  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │             │     │
│  │  │  │  Agent │  │  │  │  Agent │  │  │  │  Agent │  │             │     │
│  │  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │             │     │
│  │  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │             │     │
│  │  │  │ NVIDIA │  │  │  │HUAWEI  │  │  │  │ HYGON  │  │             │     │
│  │  │  │ GPU    │  │  │  │ NPU    │  │  │  │ DCU    │  │             │     │
│  │  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │             │     │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │     │
│  └────────────────────────────────────────────────────────────────────┘     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. 安全设计

### 7.1 权限模型

```yaml
# RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: hcs-node-agent
rules:
- apiGroups: ["hcs.io"]
  resources: ["computenodes", "computenodes/status"]
  verbs: ["get", "list", "create", "update", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch"]
```

### 7.2 准入控制

- Webhook TLS 认证
- Pod 注解防篡改
- 资源配额强制执行
- 设备访问隔离

---

## 8. 可观测性

### 8.1 指标

```go
// Prometheus 指标
var (
    vramAllocation = prometheus.NewGaugeVec(...)
    computeUsage   = prometheus.NewGaugeVec(...)
    healthScore    = prometheus.NewGaugeVec(...)
    schedulingLatency = prometheus.NewHistogram(...)
)
```

### 8.2 日志

```
级别    使用场景
----    --------------------------------------------------
DEBUG   硬件检测详情、调度决策过程
INFO    资源上报、Pod 调度事件
WARN    亚健康检测、资源不足
ERROR   硬件故障、调度失败
```

---

## 9. 性能指标

| 指标 | 目标值 | 测量方法 |
|------|--------|----------|
| 调度延迟 | < 100ms | histogram |
| 资源上报间隔 | 10s | ticker |
| Webhook 处理 | < 50ms | histogram |
| 拦截器开销 | < 5% | benchmark |
