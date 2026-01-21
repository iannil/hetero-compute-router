# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库中工作时提供指导。

## 项目概述

**Hetero-Compute-Router (HCS)** 是一个基于 Kubernetes 的异构算力虚拟化与适配层。它抽象屏蔽硬件差异（NVIDIA、华为昇腾、海光、寒武纪），实现 AI 工作负载的"一次编写，随处运行"。

### 核心理念：软硬解耦

传统方式：用户申请 `nvidia.com/gpu` -> 调度到 NVIDIA 节点
HCS：用户申请 `ai.compute/vram: 16Gi` -> 调度器分析集群现状 -> 动态分配 NVIDIA A100 或华为昇腾 910B -> 自动注入对应驱动和环境变量

## 架构：三层解耦模型

### 1. 统一资源抽象层 (URA)

- 部署在每个工作节点上的 **Node-Agent**
- 绕过厂商特定的 Device Plugin，直接调用底层工具（NVML、DSMI、CNMon）
- 上报"算力指纹"：
  - 显存大小 (VRAM)
  - 计算能力 (FP16/FP32 TFLOPS)
  - 互联拓扑 (NVLink/HCCS/PCIe)
  - 健康评分（用于国产硬件的容错）

### 2. 拓扑感知调度器

- 扩展 Kubernetes 调度框架
- 评分插件：
  - 亲和性打分（优先高速互联节点）
  - 碎片整理优化
  - 等效算力替换

### 3. 运行时注入器（核心杀手锏）

- 使用 Kubernetes Mutating Admission Webhook
- 自动完成：
  - 挂载厂商特定的驱动
  - 注入环境变量
  - 可动态替换不同推理引擎的入口点

## 规划技术栈

- **Go**：用于 Kubernetes 操作器、CRD、调度器扩展
- **Rust**：用于性能关键组件（拦截器控制逻辑）
- **C/C++**：用于 API 劫持库 (`libhcs_interceptor.so`)
- **Kubernetes**：CRD、调度框架、变更准入 Webhook
- **eBPF**：用于硬件监控和故障检测

## 开发阶段

### Phase 1: The Observer（全知之眼，MVP）

- 支持 NVIDIA 和华为昇腾的 Node-Agent
- 统一算力上报的 CRD 定义
- 基础调度逻辑

### Phase 2: The Router（算力路由）

- 具有"算力汇率"换算能力的调度器
- 用于环境变量注入的 Webhook (`LD_LIBRARY_PATH`)

### Phase 3: The Virtualizer（算力虚拟化）

- `libhcs_interceptor.so` 通过 API 劫持实现显存配额强制
- 动态镜像重定

## 关键技术挑战

1. **软件定义显存切分**：通用 API 劫持库，使用 `LD_PRELOAD` 拦截 `cudaMalloc`/`aclrtMalloc` 实现配额限制，无需硬件虚拟化支持

2. **跨芯片拓扑感知**：最大团算法变体，根据互联带宽（NVLink vs HCCS vs PCIe）实现最优 Pod 放置

3. **亚健康检测**：通过 eBPF 进行微秒级 GPU 监控，检测性能下降（降频、ECC 错误）并自动隔离受影响节点

4. **跨架构性能归一化**：内置基准测试数据库，用于"算力货币"换算（如多少张摩尔线程 S4000 = 1 张 A800）

## 设计原则

- **兼容 Volcano/KubeFlow**：HCS 作为 K8s 插件存在，而非完整的集群管理器替代品
- **框架支持**：优先适配 PyTorch、PaddlePaddle、MindSpore
- **消除厂商锁定**：实现硬件厂商间的无缝切换，零代码修改

## 资源请求格式

用户请求抽象资源而非厂商特定资源：

```yaml
resources:
  requests:
    ai.compute/vram: 16Gi
    ai.compute/tflops-fp16: "100"
```

调度器根据可用性和拓扑将其转换为实际硬件分配。
