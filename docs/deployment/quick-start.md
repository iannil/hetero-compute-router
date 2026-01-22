# HCS 快速开始指南

本指南帮助你在 5-10 分钟内完成 HCS (Hetero-Compute-Router) 的部署。

## 前置条件

在开始之前，请确保满足以下条件：

- Kubernetes 集群 1.24+
- Helm 3.10+
- kubectl 已配置并可访问集群
- 至少一个 GPU 节点（NVIDIA 或华为昇腾）

详细的前置条件请参阅 [prerequisites.md](prerequisites.md)。

## 安装步骤

### 方式一：使用 OCI Registry（推荐）

```bash
# 1. 安装 HCS
helm install hcs oci://ghcr.io/zrs-io/hetero-compute-router/charts/hcs \
  --namespace hcs-system \
  --create-namespace

# 2. 验证安装
kubectl get pods -n hcs-system
```

### 方式二：从源码安装

```bash
# 1. 克隆仓库
git clone https://github.com/zrs-io/hetero-compute-router.git
cd hetero-compute-router

# 2. 安装 CRD
kubectl apply -f config/crd/

# 3. 使用 Helm 安装
helm install hcs ./chart/hcs \
  --namespace hcs-system \
  --create-namespace
```

### 方式三：开发环境安装

```bash
# 使用开发配置（单副本，debug 日志）
helm install hcs ./chart/hcs \
  -f ./chart/hcs/values-dev.yaml \
  --namespace hcs-dev \
  --create-namespace
```

### 方式四：生产环境安装

```bash
# 使用生产配置（高可用，资源限制）
helm install hcs ./chart/hcs \
  -f ./chart/hcs/values-prod.yaml \
  --namespace hcs-system \
  --create-namespace
```

## 验证安装

### 检查 Pod 状态

```bash
kubectl get pods -n hcs-system
```

预期输出：
```
NAME                             READY   STATUS    RESTARTS   AGE
hcs-node-agent-xxxxx             1/1     Running   0          1m
hcs-scheduler-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
hcs-webhook-xxxxxxxxxx-xxxxx     1/1     Running   0          1m
```

### 检查 ComputeNode 资源

```bash
kubectl get computenodes
```

预期输出（如果有 GPU 节点）：
```
NAME          VENDOR   NODE        PHASE   VRAM          AGE
gpu-node-01   nvidia   gpu-node-01 Ready   85899345920   1m
```

### 查看详细信息

```bash
kubectl describe computenode gpu-node-01
```

## 使用 HCS 调度

### 基础用法

在 Pod 中使用 HCS 调度器：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ai-workload
spec:
  schedulerName: hcs-scheduler  # 使用 HCS 调度器
  containers:
  - name: pytorch
    image: pytorch/pytorch:latest
    resources:
      requests:
        ai.compute/vram: "16Gi"        # 请求 16GB 显存
        ai.compute/tflops-fp16: "100"  # 请求 100 TFLOPS FP16 算力
```

### PyTorch 示例

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
    command: ["python", "-c", "import torch; print(torch.cuda.is_available())"]
    resources:
      requests:
        ai.compute/vram: "8Gi"
      limits:
        ai.compute/vram: "16Gi"
```

## 卸载

```bash
# 卸载 HCS
helm uninstall hcs -n hcs-system

# 删除 CRD（可选）
kubectl delete crd computenodes.hetero.zrs.io

# 删除命名空间
kubectl delete namespace hcs-system
```

## 常见问题

### Pod 一直处于 Pending 状态

1. 检查是否有可用的 GPU 节点：
   ```bash
   kubectl get computenodes
   ```

2. 检查资源请求是否超出可用容量：
   ```bash
   kubectl describe computenode <node-name>
   ```

### Webhook 连接失败

1. 检查 cert-manager 是否已安装：
   ```bash
   kubectl get pods -n cert-manager
   ```

2. 检查证书状态：
   ```bash
   kubectl get certificate -n hcs-system
   ```

### Node-Agent 无法检测 GPU

1. 检查 NVIDIA 驱动是否已安装：
   ```bash
   nvidia-smi
   ```

2. 检查 Node-Agent 日志：
   ```bash
   kubectl logs -n hcs-system -l app.kubernetes.io/component=node-agent
   ```

## 下一步

- 查看 [配置参数说明](configuration.md) 了解所有可配置项
- 查看 [前置条件](prerequisites.md) 了解详细的环境要求
- 查看 [架构设计](/docs/design/architecture.md) 了解 HCS 工作原理
