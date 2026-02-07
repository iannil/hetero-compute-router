# 项目路线图 (Roadmap)

**项目**: Hetero-Compute-Router (HCS)
**版本**: v0.2.0-beta
**更新时间**: 2026-02-07

---

## 总览

```
Phase 1: The Observer      Phase 2: The Router        Phase 3: The Virtualizer
(全知之眼)                  (算力路由)                   (算力虚拟化)
━━━━━━━━━━━━━━━━━━━━━━━   ━━━━━━━━━━━━━━━━━━━━━━━   ━━━━━━━━━━━━━━━━━━━━━━━
┌─────────────┐           ┌─────────────┐            ┌─────────────┐
│  Node-Agent │──────────▶│  Scheduler  │──────────▶│ Interceptor │
│  + CRDs     │           │  + Webhook  │            │  + eBPF     │
└─────────────┘           └─────────────┘            └─────────────┘
    ✅ 完成                    ✅ 基本完成                🔄 进行中
```

---

## 当前状态概览

| 指标 | 数值 |
|------|------|
| Go 代码行数 | ~9,500 |
| C 代码行数 | ~1,200（拦截器 + eBPF） |
| 测试覆盖率 | ~75% |
| CRD 定义数 | 1 (ComputeNode) |
| 支持硬件厂商 | 3（NVIDIA 完整、海光完整、华为 Mock） |
| eBPF 程序数 | 3 |

---

## Phase 1: The Observer (全知之眼) ✅ 已完成

**目标**: 建立统一资源抽象层，实现异构硬件的统一视图

**里程碑**: 能够在 Kubernetes 集群上看到统一的算力资源视图

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 1.1 项目初始化 | P0 | ✅ 完成 |
| 1.2 CRD 定义 | P0 | ✅ 完成 |
| 1.3 Node-Agent MVP | P0 | ✅ 完成 |
| 1.4 基础调度器 | P1 | ✅ 完成 |
| 1.5 测试与文档 | P1 | ✅ 完成 |

### 1.1 项目初始化 ✅

- [x] 创建 Go 模块结构
- [x] 配置 GitHub Actions CI
- [x] 配置 golangci-lint
- [x] Makefile 构建脚本

### 1.2 CRD 定义 ✅

- [x] ComputeNode CRD (`pkg/api/v1alpha1/types.go`)
- [x] 设备信息结构（VRAM、TFLOPS、拓扑、健康评分）
- [x] kubebuilder 生成的 deepcopy
- [x] CRD YAML 清单 (`config/crd/`)

### 1.3 Node-Agent MVP ✅

| 子任务 | 状态 |
|--------|------|
| NVIDIA NVML 检测器 | ✅ 完整实现 |
| 海光 DCU 检测器 | ✅ 完整实现 |
| 华为昇腾检测器 | 🔄 Mock 实现 |
| 寒武纪 MLU 检测器 | ❌ 待实现 |
| 算力指纹采集 | ✅ 完成 |
| CRD 上报逻辑 | ✅ 完成 |
| 健康检查 | ✅ 完成 |

### 1.4 基础调度器 ✅

- [x] Filter 插件：根据算力需求过滤节点
- [x] Score 插件：根据算力余量、健康评分、拓扑打分
- [x] Reserve 插件：预留算力资源
- [x] 算力汇率归一化集成

### 1.5 测试与文档 ✅

- [x] 单元测试覆盖率 ~75%
- [x] 集成测试框架 (kind cluster)
- [x] Helm Chart 部署文档
- [x] API 文档

**Phase 1 交付物**: ✅ 全部完成
- [x] 可部署的 Node-Agent
- [x] 可工作的调度插件
- [x] 完整的 CRD 定义
- [x] Helm Chart
- [x] 部署文档

---

## Phase 2: The Router (算力路由) ✅ 基本完成

**目标**: 实现运行时环境自动注入，支持跨厂商硬件切换

**里程碑**: 同一 Docker 镜像能在 NVIDIA 和华为昇腾节点上运行

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 2.1 算力汇率换算 | P0 | ✅ 完成 |
| 2.2 Admission Webhook | P0 | ✅ 完成 |
| 2.3 驱动注入机制 | P0 | ✅ 完成 |
| 2.4 环境变量注入 | P1 | ✅ 完成 |
| 2.5 VRAM 拦截器 | P0 | ✅ 完成 |
| 2.6 跨厂商兼容测试 | P1 | ❌ 待开始 |

### 2.1 算力汇率换算 ✅

`pkg/exchange/calculator.go` 实现：

- [x] 15+ 硬件 Profile（NVIDIA、华为、海光）
- [x] 算力归一化算法
- [x] 等效算力替换逻辑
- [x] 基准模型配置

内置 Profile：
```
nvidia/A100-80G, nvidia/A100-40G, nvidia/A800-80G
nvidia/H100-80G, nvidia/V100-32G, nvidia/T4
huawei/Ascend910B, huawei/Ascend910A, huawei/Ascend310
hygon/DCU-Z100, hygon/DCU-Z100L
```

### 2.2 Admission Webhook ✅

`pkg/webhook/` 实现：

- [x] Mutating Webhook Handler
- [x] Pod 算力请求解析
- [x] 硬件类型检测
- [x] 运行时注入逻辑

### 2.3 驱动注入机制 ✅

`pkg/webhook/profiles.go` 支持：

| 硬件厂商 | 驱动路径 | 环境变量 | 状态 |
|----------|----------|----------|------|
| NVIDIA | /usr/local/nvidia | NVIDIA_VISIBLE_DEVICES | ✅ |
| 华为昇腾 | /usr/local/Ascend | ASCEND_VISIBLE_DEVICES | ✅ |
| 海光 | /opt/hygon | HIP_VISIBLE_DEVICES | ✅ |
| 寒武纪 | /usr/local/cambricon | MLU_VISIBLE_DEVICES | ✅ |

### 2.4 环境变量注入 ✅

- [x] LD_PRELOAD 注入（libhcs_interceptor.so）
- [x] HCS_VRAM_QUOTA 注入
- [x] 厂商特定设备选择变量
- [x] Volume 挂载

### 2.5 VRAM 拦截器 ✅

`pkg/interceptor/libhcs_interceptor.c`（913 行）：

- [x] CUDA API 拦截（cudaMalloc, cudaFree, cudaMemGetInfo, cudaMallocManaged）
- [x] ACL API 拦截（aclrtMalloc, aclrtFree, aclrtGetMemInfo）
- [x] HIP API 拦截（hipMalloc, hipFree, hipMemGetInfo）
- [x] 配额强制执行
- [x] 分配跟踪和统计

### 2.6 跨厂商兼容测试 ❌ 待开始

- [ ] PyTorch + CUDA 测试
- [ ] PyTorch + Ascend 测试
- [ ] 同一镜像跨硬件验证
- [ ] 性能基准测试

**Phase 2 交付物**:
- [x] 可工作的 Admission Webhook
- [x] 驱动注入机制
- [x] VRAM 拦截器
- [ ] 跨厂商兼容验证报告

---

## Phase 3: The Virtualizer (算力虚拟化) 🔄 进行中

**目标**: 实现软件定义的显存切分和亚健康检测

**里程碑**: 单张 GPU 可安全运行多个容器，实现显存隔离

| 任务 | 优先级 | 状态 |
|------|--------|------|
| 3.1 API 劫持库 | P0 | ✅ 完成 |
| 3.2 eBPF 监控框架 | P0 | ✅ 框架完成 |
| 3.3 eBPF 程序编译 | P1 | ❌ 待开始 |
| 3.4 动态镜像重定 | P1 | ❌ 待开始 |
| 3.5 亚健康检测与隔离 | P1 | 🔄 部分完成 |
| 3.6 自愈机制 | P2 | ❌ 待开始 |
| 3.7 生产验证 | P0 | ❌ 待开始 |

### 3.1 API 劫持库 ✅ 完成

`libhcs_interceptor.so` 架构：

```
┌─────────────────────────────────────────┐
│         Application (PyTorch)           │
└─────────────────┬───────────────────────┘
                  │ cudaMalloc() / aclrtMalloc() / hipMalloc()
                  ▼
┌─────────────────────────────────────────┐
│   libhcs_interceptor.so (LD_PRELOAD)    │
│  ┌───────────────────────────────────┐  │
│  │  dlsym Hook                       │  │
│  │  ├─ cudaMalloc ──────► wrapper    │  │
│  │  ├─ cudaFree ────────► wrapper    │  │
│  │  ├─ aclrtMalloc ─────► wrapper    │  │
│  │  ├─ aclrtFree ───────► wrapper    │  │
│  │  ├─ hipMalloc ───────► wrapper    │  │
│  │  └─ hipFree ─────────► wrapper    │  │
│  └───────────────────────────────────┘  │
│         │                                │
│         ▼                                │
│  ┌───────────────────────────────────┐  │
│  │  Quota Enforcement                │  │
│  │  - 维护全局显存计数器              │  │
│  │  - 检查配额限制                    │  │
│  │  - 返回 OOM 或转发调用             │  │
│  └───────────────────────────────────┘  │
└─────────────────┬───────────────────────┘
                  ▼
┌─────────────────────────────────────────┐
│      Vendor Driver (CUDA/Ascend/HIP)    │
└─────────────────────────────────────────┘
```

实现状态：
- [x] CUDA API 拦截
- [x] Ascend ACL API 拦截
- [x] Hygon HIP API 拦截
- [x] 配额强制执行
- [x] 分配跟踪

### 3.2 eBPF 监控框架 ✅ 框架完成

`pkg/monitoring/ebpf/` 实现：

- [x] EBPFManager 管理器
- [x] HealthAnalyzer 健康分析器（趋势分析、预测性评分）
- [x] 事件类型定义（GPUEvent, PCIeEvent, HealthEvent）
- [x] 回退到轮询模式

### 3.3 eBPF 程序 🔄 部分完成

`pkg/monitoring/ebpf/programs/` 包含：

| 程序 | 文件 | 状态 |
|------|------|------|
| GPU 监控 | gpu_monitor.c | ✅ 已编写 |
| PCIe 监控 | pcie_monitor.c | ✅ 已编写 |
| 健康事件 | health_events.c | ✅ 已编写 |

待完成：
- [ ] eBPF 程序编译集成到 Makefile
- [ ] cilium/ebpf 加载集成
- [ ] Tracepoint 附加

### 3.4 动态镜像重定 ❌ 待开始

```yaml
# 计划中的镜像重定策略
imageRebinding:
  enabled: true
  baseImages:
    nvidia: registry.hcs.io/runtime/pytorch-cuda:2.0
    huawei: registry.hcs.io/runtime/pytorch-ascend:2.0
    hygon: registry.hcs.io/runtime/pytorch-hygon:2.0
```

- [ ] InitContainer 模式实现
- [ ] CSI Volume 模式实现
- [ ] 镜像缓存优化

### 3.5 亚健康检测与隔离 🔄 部分完成

已完成：
- [x] HealthAnalyzer 趋势分析
- [x] 预测性故障检测
- [x] 健康评分计算

待完成：
- [ ] 自动节点隔离
- [ ] 告警通知
- [ ] 工作负载迁移

### 3.6 自愈机制 ❌ 待开始

- [ ] Checkpoint 自动发现
- [ ] 故障 Pod 自动迁移
- [ ] 异构备用节点支持

### 3.7 生产验证 ❌ 待开始

- [ ] 真实硬件环境测试
- [ ] 长时间稳定性测试
- [ ] 压力测试
- [ ] 性能回归测试

**Phase 3 交付物**:
- [x] API 劫持库
- [x] eBPF 框架
- [ ] eBPF 程序编译
- [ ] 动态镜像重定
- [ ] 亚健康检测与自愈
- [ ] 生产级部署文档

---

## 版本规划

| 版本 | 发布时间 | 主要功能 | 状态 |
|------|----------|----------|------|
| v0.1.0-alpha | 2026 Q1 | Phase 1 MVP | ✅ 完成 |
| v0.2.0-beta | 2026 Q2 | Phase 2 完成 | 🔄 当前 |
| v0.3.0-beta | 2026 Q3 | Phase 3 完成 | 📅 计划 |
| v1.0.0 | 2026 Q4 | 生产就绪版本 | 📅 计划 |

---

## 下一步行动（优先级排序）

### P0 - 紧急

1. **寒武纪 MLU 检测器实现**
   - 参考海光 DCU 检测器实现
   - 需要 Cambricon SDK 文档

2. **真实硬件环境测试**
   - 云环境租赁（NVIDIA A100、华为 910B）
   - 海光 DCU 环境获取

### P1 - 重要

3. **eBPF 程序编译集成**
   - 添加 Makefile 目标
   - 集成 cilium/ebpf

4. **跨厂商兼容性测试**
   - PyTorch 测试套件
   - 性能基准对比

### P2 - 正常

5. **动态镜像重定**
   - InitContainer 模式 POC

6. **亚健康节点自动隔离**
   - 与 Kubernetes Node 状态集成

---

## 风险与阻塞

| 风险项 | 影响 | 缓解措施 | 状态 |
|--------|------|----------|------|
| 国产硬件 SDK 文档不全 | 中 | Mock 测试 + 社区支持 | 🔄 进行中 |
| eBPF 内核版本要求 | 低 | 已实现回退到轮询模式 | ✅ 已缓解 |
| 真实硬件测试环境 | 中 | 云环境租赁 | 📅 待解决 |
| 寒武纪 SDK 获取 | 中 | 联系厂商或使用开源实现 | 📅 待解决 |

---

## 依赖关系

```
外部依赖:
├── Kubernetes >= 1.24
├── containerd >= 1.6
├── Go >= 1.21
├── C Compiler (gcc/clang)
├── CUDA Toolkit >= 11.8 (可选)
├── CANN >= 7.0 (可选)
├── ROCm/HIP (可选)
└── eBPF 相关工具 (bcc, libbpf) (可选)
```

---

## 变更历史

| 日期 | 变更内容 |
|------|----------|
| 2026-02-07 | 更新实际实现状态，Phase 1 标记完成，Phase 2 基本完成 |
| 2026-01-23 | 添加海光 DCU 检测器、eBPF 框架、集成测试 |
| 2026-01-21 | 初始版本 |
