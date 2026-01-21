# 项目路线图 (Roadmap)

**项目**: Hetero-Compute-Router (HCS)
**版本**: v0.1.0-alpha
**更新时间**: 2026-01-21

---

## 总览

```
Phase 1: The Observer      Phase 2: The Router        Phase 3: The Virtualizer
(全知之眼)                  (算力路由)                   (算力虚拟化)
━━━━━━━━━━━━━━━━━━━━━━━   ━━━━━━━━━━━━━━━━━━━━━━━   ━━━━━━━━━━━━━━━━━━━━━━━
┌─────────────┐           ┌─────────────┐            ┌─────────────┐
│  Node-Agent │──────────▶│  Scheduler  │──────────▶│ Interceptor │
│  + CRDs     │           │  + Webhook  │            │  + Image    │
└─────────────┘           └─────────────┘            │   Rebinding │
                                                       └─────────────┘
    MVP                       生产可用                    完整形态
```

---

## Phase 1: The Observer (全知之眼)

**目标**: 建立统一资源抽象层，实现异构硬件的统一视图

**里程碑**: 能够在 Kubernetes 集群上看到统一的算力资源视图

| 任务 | 优先级 | 预估工作量 | 状态 |
|------|--------|-----------|------|
| 1.1 项目初始化 | P0 | 2天 | ⬜ 未开始 |
| 1.2 CRD 定义 | P0 | 3天 | ⬜ 未开始 |
| 1.3 Node-Agent MVP | P0 | 10天 | ⬜ 未开始 |
| 1.4 基础调度器 | P1 | 8天 | ⬜ 未开始 |
| 1.5 测试与文档 | P1 | 5天 | ⬜ 未开始 |

### 1.1 项目初始化 (2天)

- [ ] 创建 Go 模块结构
  ```
  hetero-compute-router/
  ├── cmd/
  │   ├── node-agent/
  │   ├── scheduler/
  │   └── webhook/
  ├── pkg/
  │   ├── api/
  │   ├── detectors/
  │   └── collectors/
  ├── config/
  └── go.mod
  ```
- [ ] 创建 Rust 项目结构 (用于拦截器控制逻辑)
- [ ] 创建 C/C++ 项目结构 (用于 API 劫持库)
- [ ] 配置 pre-commit hooks
- [ ] 配置 GitHub Actions CI

### 1.2 CRD 定义 (3天)

```yaml
# ComputeNode - 节点算力资源
apiVersion: hcs.io/v1alpha1
kind: ComputeNode
metadata:
  name: node-1
spec:
  vendor: nvidia  # nvidia, huawei, hygon, cambricon
  devices:
    - model: A100-80G
      vram: 80Gi
      vramAllocatable: 80Gi
      compute:
        fp16: "312"  # TFLOPS
        fp32: "19.5" # TFLOPS
      topology:
        interconnect: nvlink
        peers: [gpu-1, gpu-2]
      healthScore: 100
```

CRD 列表:
- [ ] ComputeNode - 节点算力资源
- [ ] ComputeTask - 任务算力请求
- [ ] ComputeTopology - 集群拓扑关系
- [ ] ComputeQuota - 算力配额

### 1.3 Node-Agent MVP (10天)

| 子任务 | 详情 |
|--------|------|
| NVIDIA 检测 | 使用 NVML 库检测 GPU 信息 |
| 华为昇腾检测 | 使用 DSMI 库检测 NPU 信息 |
| 算力指纹采集 | VRAM、算力、拓扑、健康状态 |
| CRD 上报 | 定时更新 ComputeNode 资源 |
| 健康检查 | 实时监控硬件状态 |

### 1.4 基础调度器 (8天)

- [ ] 实现 Filter 插件：根据算力需求过滤节点
- [ ] 实现 Score 插件：根据算力余量和拓扑打分
- [ ] 实现 Reserve 插件：预留算力资源
- [ ] 实现 Bind 插件：绑定 Pod 到节点

### 1.5 测试与文档 (5天)

- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试 (kind cluster)
- [ ] API 文档生成
- [ ] 部署文档编写

**Phase 1 交付物**:
- 可部署的 Node-Agent
- 可工作的调度插件
- 完整的 CRD 定义
- 部署文档

---

## Phase 2: The Router (算力路由)

**目标**: 实现运行时环境自动注入，支持跨厂商硬件切换

**里程碑**: 同一 Docker 镜像能在 NVIDIA 和华为昇腾节点上运行

| 任务 | 优先级 | 预估工作量 | 状态 |
|------|--------|-----------|------|
| 2.1 算力汇率换算 | P0 | 5天 | ⬜ 未开始 |
| 2.2 Admission Webhook | P0 | 8天 | ⬜ 未开始 |
| 2.3 驱动注入机制 | P0 | 5天 | ⬜ 未开始 |
| 2.4 环境变量注入 | P1 | 3天 | ⬜ 未开始 |
| 2.5 跨厂商兼容测试 | P1 | 7天 | ⬜ 未开始 |

### 2.1 算力汇率换算 (5天)

```yaml
# 算力汇率配置
exchangeRates:
  baseUnit: nvidia-a100-100%
  conversions:
    nvidia-a100: 1.0
    nvidia-a800: 0.7
    huawei-ascend910b: 0.6
    hygon-dc100: 0.5
```

- [ ] 实现算力归一化算法
- [ ] 实现等效算力替换逻辑
- [ ] 支持用户自定义汇率

### 2.2 Admission Webhook (8天)

```go
// Webhook 逻辑流程
func (w *Webhook) Handle(pod *v1.Pod) (*v1.Pod, error) {
    1. 解析 Pod 算力请求 (ai.compute/vram, ai.compute/tflops-fp16)
    2. 调用 Scheduler 获取目标节点
    3. 查询目标节点硬件类型
    4. 修改 Pod Spec:
       - 注入驱动挂载
       - 注入环境变量
       - 可选: 替换基础镜像
    5. 返回修改后的 Pod
}
```

### 2.3 驱动注入机制 (5天)

| 硬件厂商 | 驱动路径 | 环境变量 |
|----------|----------|----------|
| NVIDIA | /usr/local/nvidia | NVIDIA_VISIBLE_DEVICES, LD_LIBRARY_PATH |
| 华为昇腾 | /usr/local/Ascend | ASCEND_VISIBLE_DEVICES, LD_LIBRARY_PATH |
| 海光 | /opt/hygon | HYGON_VISIBLE_DEVICES, LD_LIBRARY_PATH |
| 寒武纪 | /usr/local/cambricon | CAMBRICON_VISIBLE_DEVICES, LD_LIBRARY_PATH |

### 2.4 环境变量注入 (3天)

- [ ] LD_LIBRARY_PATH 注入
- [ ] 厂商特定设备选择变量
- [ ] 框架配置变量 (NCCL, HCCL 等)
- [ ] 性能调优变量

### 2.5 跨厂商兼容测试 (7天)

- [ ] PyTorch + CUDA 测试
- [ ] PyTorch + Ascend 测试
- [ ] 同一镜像跨硬件验证
- [ ] 性能基准测试

**Phase 2 交付物**:
- 可工作的 Admission Webhook
- 驱动注入机制
- 跨厂商兼容验证报告

---

## Phase 3: The Virtualizer (算力虚拟化)

**目标**: 实现软件定义的显存切分和动态镜像重定

**里程碑**: 单张 GPU 可安全运行多个容器，实现显存隔离

| 任务 | 优先级 | 预估工作量 | 状态 |
|------|--------|-----------|------|
| 3.1 API 劫持库 | P0 | 15天 | ⬜ 未开始 |
| 3.2 动态镜像重定 | P1 | 10天 | ⬜ 未开始 |
| 3.3 亚健康检测 | P1 | 8天 | ⬜ 未开始 |
| 3.4 自愈机制 | P2 | 10天 | ⬜ 未开始 |
| 3.5 生产验证 | P0 | 15天 | ⬜ 未开始 |

### 3.1 API 劫持库 (15天)

```
libhcs_interceptor.so 架构:
┌─────────────────────────────────────────┐
│         Application (PyTorch)           │
└─────────────────┬───────────────────────┘
                  │ cudaMalloc()
                  ▼
┌─────────────────────────────────────────┐
│   libhcs_interceptor.so (LD_PRELOAD)    │
│  ┌───────────────────────────────────┐  │
│  │  dlsym Hook                       │  │
│  │  ├─ cudaMalloc ──────► wrapper    │  │
│  │  ├─ cudaFree ────────► wrapper    │  │
│  │  ├─ aclrtMalloc ─────► wrapper    │  │
│  │  └─ aclrtFree ───────► wrapper    │  │
│  └───────────────────────────────────┘  │
│         │                                 │
│         ▼                                 │
│  ┌───────────────────────────────────┐  │
│  │  Quota Enforcement                │  │
│  │  - 维护全局显存计数器              │  │
│  │  - 检查配额限制                    │  │
│  │  - 返回 OOM 或转发调用             │  │
│  └───────────────────────────────────┘  │
└─────────────────┬───────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────┐
│      Vendor Driver (CUDA/Ascend)        │
└─────────────────────────────────────────┘
```

- [ ] CUDA API 拦截 (cudaMalloc, cudaFree)
- [ ] Ascend API 拦截 (aclrtMalloc, aclrtFree)
- [ ] 配额强制执行
- [ ] 多容器安全隔离

### 3.2 动态镜像重定 (10天)

```yaml
# 镜像重定策略
imageRebinding:
  enabled: true
  baseImages:
    nvidia: registry.hcs.io/runtime/pytorch-cuda:2.0
    huawei: registry.hcs.io/runtime/pytorch-ascend:2.0
    hygon: registry.hcs.io/runtime/pytorch-hygon:2.0
  codeLayer:
    type: initContainer  # or csi-volume
```

- [ ] InitContainer 模式实现
- [ ] CSI Volume 模式实现
- [ ] 镜像缓存优化

### 3.3 亚健康检测 (8天)

- [ ] eBPF PCIe 监控
- [ ] 性能降级检测
- [ ] ECC 错误监控
- [ ] 自动隔离机制

### 3.4 自愈机制 (10天)

- [ ] Checkpoint 自动发现
- [ ] 故障 Pod 自动迁移
- [ ] 异构备用节点支持

### 3.5 生产验证 (15天)

- [ ] 长时间稳定性测试
- [ ] 压力测试
- [ ] 性能回归测试
- [ ] 用户验收测试

**Phase 3 交付物**:
- 可用的 API 劫持库
- 动态镜像重定功能
- 亚健康检测与自愈
- 生产级部署文档

---

## 版本规划

| 版本 | 发布时间 | 主要功能 |
|------|----------|----------|
| v0.1.0-alpha | Q2 2026 | Phase 1 MVP |
| v0.2.0-beta | Q3 2026 | Phase 2 完成 |
| v0.3.0-beta | Q4 2026 | Phase 3 完成 |
| v1.0.0 | Q1 2027 | 生产就绪版本 |

---

## 依赖关系

```
外部依赖:
├── Kubernetes >= 1.25
├── containerd >= 1.6
├── Go >= 1.21
├── Rust >= 1.70
├── CUDA Toolkit >= 11.8
├── CANN >= 7.0
└── eBPF 相关工具 (bcc, libbpf)
```
