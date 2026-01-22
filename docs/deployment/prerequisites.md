# HCS 前置条件

本文档列出了部署 HCS (Hetero-Compute-Router) 所需的所有前置条件。

## Kubernetes 集群

### 版本要求

| 组件 | 最低版本 | 推荐版本 |
|------|----------|----------|
| Kubernetes | 1.24 | 1.28+ |
| kubectl | 1.24 | 与集群版本匹配 |
| Helm | 3.10 | 3.13+ |

### 集群功能要求

- **RBAC**: 必须启用
- **Admission Webhooks**: 必须支持 MutatingAdmissionWebhook
- **CRD**: 必须支持 CustomResourceDefinition v1

### 验证集群版本

```bash
# 检查 Kubernetes 版本
kubectl version --short

# 检查是否支持 Admission Webhooks
kubectl api-versions | grep admissionregistration.k8s.io/v1
```

## 组件依赖

### cert-manager（推荐）

HCS Webhook 需要 TLS 证书。推荐使用 cert-manager 自动管理证书。

**版本要求**: 1.12+

**安装 cert-manager**:
```bash
# 使用 Helm 安装
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

**验证安装**:
```bash
kubectl get pods -n cert-manager
```

> **注意**: 如果不使用 cert-manager，需要手动管理 Webhook 的 TLS 证书。

### metrics-server（可选）

用于资源监控和 HPA 支持。

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

## 硬件要求

### NVIDIA GPU

| 组件 | 最低版本 | 推荐版本 |
|------|----------|----------|
| NVIDIA 驱动 | 450.x | 525.x+ |
| CUDA | 11.0 | 12.0+ |

**验证 NVIDIA 环境**:
```bash
# 在 GPU 节点上运行
nvidia-smi
```

**预期输出示例**:
```
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.85.12    Driver Version: 525.85.12    CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|===============================+======================+======================|
|   0  NVIDIA A100-SXM4-80GB  On| 00000000:07:00.0 Off |                    0 |
| N/A   29C    P0    62W / 400W |      0MiB / 81920MiB |      0%      Default |
+-------------------------------+----------------------+----------------------+
```

### 华为昇腾 NPU

| 组件 | 最低版本 | 推荐版本 |
|------|----------|----------|
| CANN | 5.0 | 7.0+ |
| 昇腾驱动 | 21.0.2 | 最新版 |

**验证昇腾环境**:
```bash
# 在 NPU 节点上运行
npu-smi info
```

### 海光 DCU（规划中）

| 组件 | 最低版本 |
|------|----------|
| ROCm | 5.0 |
| HIP | 5.0 |

### 寒武纪 MLU（规划中）

| 组件 | 最低版本 |
|------|----------|
| CNToolkit | 2.0 |
| CNMON | 2.0 |

## 网络要求

### 端口

| 组件 | 端口 | 协议 | 说明 |
|------|------|------|------|
| Webhook | 8443 | HTTPS | Admission Webhook 服务 |
| Webhook Metrics | 8080 | HTTP | Prometheus 指标 |
| Scheduler | 10259 | HTTP | 健康检查和指标 |

### DNS 解析

Webhook 依赖 Kubernetes 集群内 DNS 解析服务名称。确保：

```bash
# 验证 DNS 解析
kubectl run test-dns --image=busybox --rm -it --restart=Never -- nslookup kubernetes.default
```

### 网络策略

如果集群启用了 NetworkPolicy，需要允许以下流量：

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-webhook
  namespace: hcs-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: webhook
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - port: 8443
      protocol: TCP
```

## 资源要求

### 最小资源配置

| 组件 | CPU 请求 | 内存请求 | CPU 限制 | 内存限制 |
|------|----------|----------|----------|----------|
| Node-Agent | 100m | 128Mi | 500m | 512Mi |
| Scheduler | 200m | 256Mi | 1000m | 1Gi |
| Webhook | 100m | 128Mi | 500m | 512Mi |

### 生产环境推荐配置

| 组件 | CPU 请求 | 内存请求 | CPU 限制 | 内存限制 | 副本数 |
|------|----------|----------|----------|----------|--------|
| Node-Agent | 100m | 128Mi | 500m | 512Mi | 每节点1个 |
| Scheduler | 500m | 512Mi | 2000m | 2Gi | 3 |
| Webhook | 200m | 256Mi | 1000m | 1Gi | 3 |

## 权限要求

### 安装权限

安装 HCS 需要以下 Kubernetes 权限：

- 创建 Namespace
- 创建 CustomResourceDefinition
- 创建 ClusterRole 和 ClusterRoleBinding
- 创建 MutatingWebhookConfiguration
- 创建 Deployment, DaemonSet, Service 等

### RBAC 验证

```bash
# 检查当前用户是否有集群管理员权限
kubectl auth can-i '*' '*' --all-namespaces
```

## 前置条件检查脚本

运行以下脚本验证所有前置条件：

```bash
#!/bin/bash

echo "=== HCS Prerequisites Check ==="

# Kubernetes version
echo -n "Kubernetes version: "
kubectl version --short 2>/dev/null | grep Server || echo "FAILED"

# Helm version
echo -n "Helm version: "
helm version --short 2>/dev/null || echo "FAILED"

# Admission webhooks
echo -n "Admission webhooks: "
kubectl api-versions | grep -q admissionregistration.k8s.io/v1 && echo "OK" || echo "FAILED"

# cert-manager
echo -n "cert-manager: "
kubectl get pods -n cert-manager 2>/dev/null | grep -q Running && echo "OK" || echo "NOT INSTALLED (optional)"

# GPU nodes
echo -n "GPU nodes: "
kubectl get nodes -l nvidia.com/gpu.present=true 2>/dev/null | grep -q Ready && echo "OK" || echo "NONE FOUND"

echo "=== Check Complete ==="
```

## 下一步

满足所有前置条件后，请参阅 [快速开始指南](quick-start.md) 进行安装。
