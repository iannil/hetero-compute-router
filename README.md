# HETERO-COMPUTE-ROUTER

针对 异构算力统一调度器 (Hetero-Compute-Scheduler)，要在一个已经存在如 Volcano（华为）、YuniKorn、Kube-Scheduler 以及各种厂商私有 Device Plugin 的红海中脱颖而出，“差异化”是核心命门。

现有的方案大多解决了“怎么把Pod塞进节点”的问题，但没有很好地解决“业务代码如何无感适配不同硬件”以及“国产卡碎片化管理”的痛点。

以下是打造具有极高竞争力的差异化技术方案：

---

### 项目定位：不仅仅是调度器，而是“算力虚拟化与适配层”

核心差异化理念： 软硬解耦。
传统的调度器：用户申请 `nvidia.com/gpu` -> 调度到 NVIDIA 节点。
HCS (Hetero-Compute-Scheduler)： 用户申请 `ai.compute/vram: 16Gi` -> 调度器分析集群现状 -> 动态决定给用户分配 NVIDIA A100 还是 华为 Ascend 910B -> 自动注入对应的驱动库和环境变量。

---

### 1. 架构设计：三层解耦模型

#### 第一层：统一资源抽象层 (Unified Resource Abstraction, URA)

现状痛点： K8s 里资源名称不仅碎片化（`nvidia.com/gpu`, `huawei.com/Ascend910`），而且颗粒度粗。
HCS 方案： 抛弃厂商特定的 Key，建立一套标准的 CRD (Custom Resource Definition)。

* 自定义节点发现 Agent (Node-Agent)：
  * 部署在每个节点，绕过厂商的 Device Plugin，直接调用底层工具（NVML, DSMI, CNMon）。
  * 差异化功能： 不仅仅上报“有几张卡”，而是上报“算力指纹”：
    * 显存大小 (VRAM)
    * 计算能力 (FP16/FP32 TFLOPS)
    * 互联拓扑 (NVLink/HCCS/PCIe) —— *这对分布式训练至关重要*。
    * 健康评分 (Health Score) —— *国产卡容易掉卡，需实时标记亚健康状态*。

#### 第二层：动态感知调度器 (Topology-Aware Scheduler)

现状痛点： 普通调度器不懂 AI 拓扑，可能把需要通信的两个 Pod 调度到了跨机房或者没有 RDMA 连接的节点上。
HCS 方案： 基于 K8s Scheduling Framework 扩展。

* 评分策略 (Scoring Plugins)：
  * 亲和性打分： 优先将同一任务的 Pod 调度到具有高速互联（如 RoCE, IB）的节点组。
  * 碎片整理： 优先填满已有碎片的节点，而不是开启新节点（Cost-Saving）。
  * 等效算力替换： 当用户申请 NVIDIA A100 但资源耗尽时，调度器能计算出 "2张 Ascend 910B ≈ 1张 A100"，并自动询问用户（通过 Annotation 策略）是否降级或跨架构迁移。

#### 第三层：运行时环境注入器 (Runtime Injector - 核心杀手锏)

现状痛点： 就算调度过去了，容器里的 CUDA 库在华为卡上跑不起来。
HCS 方案： 利用 K8s Mutating Admission Webhook 实现“运行时变身”。

* 工作流：
  1. 用户提交一个基础镜像（包含 PyTorch 框架代码，代码中做了 `if cuda: ... elif ascend: ...` 的兼容）。
  2. HCS 决定将该 Pod 调度到 华为昇腾 节点。
  3. Webhook 拦截： 修改 Pod Spec。
     * 挂载： 自动挂载 Host 上的 CANN 驱动路径 `/usr/local/Ascend` 到容器内。
     * 注入： 注入 `LD_LIBRARY_PATH` 和厂商特定的环境变量（如 `ASCEND_VISIBLE_DEVICES`）。
     * 替换： 如果是推理场景，甚至可以动态替换 Entrypoint，调用不同的推理引擎（TensorRT vs ACL）。

---

### 2. 关键技术难点与解决方案

#### 2.1 难点：如何实现显存切分（算力共享）？

国产卡显存昂贵，推理场景利用率低。NVIDIA 有 MIG/MPS，国产卡支持参差不齐。
差异化方案：软件定义的通用切分 (Software-Defined Slicing)

* 技术路径： 开发一个通用的 API Hijack Library (劫持库)。
* 实现： 类似 `vCUDA` 的原理，但做成通用接口。在容器启动时 `LD_PRELOAD` 这个库。拦截 CUDA Malloc 或 ACL Malloc 调用，限制其申请的显存上限。
* 优势： 不依赖硬件特性，让老旧的显卡或不支持虚拟化的国产卡也能实现“一张卡跑4个容器”。

#### 2.2 难点：如何处理“掉卡”与容错？

国产硬件在长时间训练中稳定性略逊于成熟产品。
差异化方案：亚秒级故障隔离与断点续训

* Node-Agent 增强： 实现微秒级的 GPU 状态轮询（通过 eBPF 监控 PCIe 错误）。
* 控制器逻辑： 一旦发现某个节点报错，HCS 立即将该节点标记为 `Tainted`，并触发上层 Operator (如 PyTorchJob) 进行 Re-schedule，结合 Checkpoint 自动迁移到备用节点（甚至可以是异构备用节点）。

#### 2.3 难点：跨架构的性能归一化

用户不知道 1张 摩尔线程 S4000 等于多少张 A800。
差异化方案：内置 Benchmark 数据库

* HCS 内置一个动态更新的 Benchmark 库（可以配合上文提到的“国产AI芯片基准测试套件”）。
* 提供一个 Unit Converter (算力换算器)。用户只需定义“我需要 100 TFLOPS FP16 算力”，HCS 自动计算出各家硬件需要的卡数。

---

### 3. 开源生态与推广策略

#### 3.1 兼容性设计（不造轮子）

* 兼容 Volcano/KubeFlow： HCS 应该作为 K8s 的一个插件存在，而不是替代整个集群管理。它可以与 Volcano 共存，接管具体的 Device Bind 环节。
* 支持主流框架： 优先适配 PyTorch, PaddlePaddle, MindSpore。

#### 3.2 路线图 (Roadmap)

* Phase 1 (MVP): 实现 Node-Agent，能够识别 NVIDIA 和 Huawei Ascend，统一上报为 CRD。实现最基础的调度逻辑。
* Phase 2 (Injector): 实现 Webhook，支持自动挂载驱动和环境变量，让同一个 Docker Image 能在两种卡上跑起来（前提是代码兼容）。
* Phase 3 (Slicing): 引入 API 劫持库，实现国产卡的显存切分共享。

### 4. 总结：为什么这个方案能拿 8.5+ 分？

* 市场紧迫性： 企业买不到 NVIDIA 卡，被迫买华为/海光/寒武纪，导致机房里全是“万国牌”硬件，运维极其痛苦。HCS 直接止痛。
* 技术壁垒： 能够把 Device Plugin、Scheduler Extender、Admission Webhook 和 eBPF 串联起来解决异构问题，技术含金量高。
* 国家战略： 符合“异构算力”、“国产芯片”的政策大方向，极易获得信创类项目和国企的青睐。

这个项目如果做出来，不仅仅是一个工具，它有可能成为国产信创云操作系统的标准组件。

---

这是一个非常出色且极具前瞻性的架构设计。你敏锐地抓住了当前国产化信创浪潮中最大的痛点：硬件碎片化与应用适配的割裂。

现有的 Volcano 或 YuniKorn 更多是在做“资源分配（Resource Allocation）”，而你提出的 HCS 实际上是在做“算力路由与适配（Compute Routing & Adaptation）”。这不仅是调度器，更是一个轻量级的 AI PaaS 核心引擎。

为了助你进一步完善这个方案，并在技术深度上形成绝对壁垒，我将从架构深化、关键实现细节、以及应对评委/客户挑战的策略三个方面进行补充和打磨。

---

### 一、 架构深化：让“解耦”更彻底

你提出的三层架构非常清晰，为了进一步增加“差异化”，建议引入“标准算力单元 (ACU, Abstract Compute Unit)”的概念。

#### 1. URA 层增强：算力归一化 (Normalization)

不要只做 CRD 抽象，要建立一套“汇率机制”。

* 设计： 定义 `1 ACU` = `NVIDIA A100 * 1% 算力`（或者以 TFLOPS 为基准）。
* Node-Agent 进阶： Agent 启动时，自动运行一组微基准测试（Micro-benchmarks，如矩阵乘法），测出当前节点的 ACU 值并上报。
* 价值： 即使是同一型号显卡，因散热、BIOS设置不同，性能也有差异。HCS 能做到基于实测性能的调度，而非基于型号标签的调度。

#### 2. Runtime Injector 增强：镜像分层与动态重组

这是最难也是最亮眼的地方。关于“代码如何无感适配”，仅仅注入环境变量是不够的（因为 PyTorch 的 CUDA 版本和 CANN 版本二进制不兼容）。

* 差异化方案：动态镜像重定 (Dynamic Image Rebinding)
  * 构建阶段： 用户不直接打大包，而是遵循 HCS 规范，构建代码层 (Code Layer) 镜像。
  * 基础设施： HCS 维护一套 Runtime Base Images（包含 PyTorch-CUDA, PyTorch-Ascend, MindSpore-Ascend 等）。
  * 调度阶段：
    1. HCS 决定将任务调度到华为节点。
    2. Webhook 拦截 Pod，将 `image` 字段替换为对应的 `Runtime Base Image (Ascend)`。
    3. 将用户的 `Code Layer` 镜像作为一个 `InitContainer` 挂载，或者通过 CSI 卷挂载到 Runtime 容器中。
  * 效果： 真正实现“一份代码，多处运行”，彻底屏蔽底层驱动库差异。

---

### 二、 关键技术难点深挖 (Implementation Deep Dive)

这里提供一些具体的实现思路，让你的技术方案看起来“不仅有想法，而且不仅能落地”。

#### 2.1 显存切分：通用劫持库 (The General Hijacker)

要做软切分，必须深入到 `libc` 和 `dlopen` 层。

* 技术栈： C/C++ (编写 `.so` 库), Rust (编写控制逻辑).
* 核心逻辑：
  * 使用 `LD_PRELOAD` 注入 `libhcs_interceptor.so`。
  * Hook `dlsym`： 这是关键。大部分 AI 框架动态加载 CUDA/Ascend 库。你需要拦截 `dlsym`，当框架请求 `cudaMalloc` 或 `aclrtMalloc` 的函数指针时，返回你自己的 wrapper 函数指针。
  * Wrapper 逻辑： 在 wrapper 中维护一个全局计数器（当前已用显存）。如果 `current + request > limit`，直接返回 OOM 错误，迫使框架进行显存释放或垃圾回收。
  * 地址空间隔离（高阶）： 如果要做到更安全，需要在 wrapper 里面把显存指针偏移量做映射（类似 MMU），但这极难。MVP 阶段做到额度限制 (Quota Enforcement) 即可。

#### 2.2 跨芯片通信拓扑感知 (Cross-Chip Topology)

不同厂商的互联技术不通（NVLink vs HCCS）。

* 方案： 建立一个 以太网/IB 互联矩阵。
* 算法： 并不是简单的 Bin-packing。使用 最大团算法 (Maximum Clique Problem) 的变种。
  * 将所有 GPU 看作图的顶点。
  * 顶点之间的边权重 = 带宽/延迟。
  * 调度目标：寻找一个子图，使得子图内部的总边权重最大，且满足 Pod 数量需求。
  * 差异点： 国产卡很多不支持 P2P 直接访问，HCS 必须感知这一点，并在 Pod 环境变量中自动设置 `NCCL_P2P_DISABLE=1` 或 `HCCL_INTRA_ROCE_ENABLE=1`，防止训练挂死。

#### 2.3 亚健康检测与“自愈” (Heuristic Self-Healing)

国产卡常见的“亚健康”不是直接挂掉，而是慢（降频、ECC 纠错频繁）。

* 实现： Node-Agent 持续采集 GPU 的 SM 利用率和 PCIe 吞吐。
* 逻辑： 如果某张卡的计算效率（TFLOPS/Power）显著低于同集群平均值超过 5 分钟 -> 自动隔离。
* 差异化： 结合 Checkpint Store。HCS 不仅重调度，还能自动找到最近的 Checkpoint 路径注入给新 Pod，实现断点无感续训。

---

### 三、 竞争策略与“故事线”

在介绍项目时，不要陷入和 Volcano 比拼“谁的调度算法更复杂”的陷阱。你的战场在运维效率和信创适配。

#### 1. 核心卖点总结 (The Pitch)

* 对开发者： “Write Once, Run on Any Hardware”。不需要关心底层是 A800 还是 910B，像用电力一样用算力。
* 对运维/CIO： “消除厂商锁定 (Vendor Lock-in)”。你可以先买 Nvidia，由于制裁买不到了，无缝切换到华为或海光，业务代码零修改，集群利用率不下降。
* 对国家/国企： “信创算力底座”。解决国产卡好买不好用的问题，屏蔽硬件差异。

#### 2. 对比分析表 (Cheat Sheet)

| 功能特性 | K8s 原生 | Volcano/YuniKorn | HCS (你的方案) |
| :--- | :--- | :--- | :--- |
| 资源粒度 | 整卡 | 静态切分 (MPS) | 动态软切分 (通用拦截) |
| 异构支持 | 仅通过 Label | 依赖 Device Plugin | 统一抽象 (URA) + 算力汇率 |
| 运行时环境 | 需手动配镜像 | 需手动配镜像 | 自动注入驱动 & 镜像重组 |
| 拓扑感知 | 无 | NUMA 亲和 | 跨机房/跨芯片互联拓扑 |
| 容错处理 | Pod 重启 | 任务重试 | 亚健康隔离 + 自动断点续训 |
| 定位 | 资源调度 | 批处理调度 | 算力虚拟化与适配层 |

### 四、 路线图建议 (Roadmap Strategy)

为了让项目看起来落地性强，建议分三步走：

1. Phase 1: The Observer (全知之眼)
   * 开发 Node-Agent，支持 Nvidia 和 Huawei。
   * 实现 CRD 定义，能在 K8s Dashboard 上看到统一的算力视图（例如：总算力 5000 ACU）。
2. Phase 2: The Router (算力路由)
   * 实现 Scheduler，支持“算力汇率”换算。
   * 实现 Webhook，支持基础的环境变量注入（`LD_LIBRARY_PATH`）。
3. Phase 3: The Virtualizer (算力虚拟化)
   * 实现 `libhcs_interceptor.so` 进行显存软隔离。
   * 实现动态镜像重定。

### 总结

你的方案如果能实现 Phase 1 和 Phase 2 的核心逻辑，并在 Demo 中展示“提交一个 PyTorch 任务，自动调度到华为卡并运行成功，且显存被限制住了”，这绝对是降维打击。这不仅仅解决了调度问题，而是解决了国产 AI 基础设施最核心的“兼容性”和“利用率”难题。
