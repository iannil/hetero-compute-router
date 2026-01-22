# HCS 配置参数说明

本文档详细说明 HCS Helm Chart 的所有配置参数。

## 全局配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `global.imageRegistry` | string | `""` | 全局镜像仓库地址 |
| `global.imagePullSecrets` | list | `[]` | 全局镜像拉取密钥 |
| `global.namespace` | string | `""` | 覆盖部署命名空间 |

## Node-Agent 配置

Node-Agent 以 DaemonSet 形式部署在所有 GPU 节点上，负责收集硬件信息。

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `nodeAgent.enabled` | bool | `true` | 是否启用 Node-Agent |
| `nodeAgent.image.repository` | string | `"ghcr.io/zrs-io/hetero-compute-router/node-agent"` | 镜像地址 |
| `nodeAgent.image.tag` | string | `""` | 镜像标签，默认使用 Chart appVersion |
| `nodeAgent.image.pullPolicy` | string | `"IfNotPresent"` | 镜像拉取策略 |
| `nodeAgent.logLevel` | string | `"info"` | 日志级别 (debug/info/warn/error) |
| `nodeAgent.reportInterval` | int | `30` | 上报间隔（秒） |
| `nodeAgent.resources.requests.cpu` | string | `"100m"` | CPU 请求 |
| `nodeAgent.resources.requests.memory` | string | `"128Mi"` | 内存请求 |
| `nodeAgent.resources.limits.cpu` | string | `"500m"` | CPU 限制 |
| `nodeAgent.resources.limits.memory` | string | `"512Mi"` | 内存限制 |
| `nodeAgent.tolerations` | list | 见下方 | 容忍配置 |
| `nodeAgent.nodeSelector` | object | `{}` | 节点选择器 |
| `nodeAgent.serviceAccount.create` | bool | `true` | 是否创建 ServiceAccount |
| `nodeAgent.serviceAccount.name` | string | `""` | ServiceAccount 名称 |

### Node-Agent 默认容忍

```yaml
tolerations:
  - operator: Exists  # 调度到所有节点（包括 master）
```

## Scheduler 配置

Scheduler 扩展 Kubernetes 调度框架，实现异构算力感知调度。

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `scheduler.enabled` | bool | `true` | 是否启用 Scheduler |
| `scheduler.replicas` | int | `1` | 副本数（生产环境建议 3） |
| `scheduler.image.repository` | string | `"ghcr.io/zrs-io/hetero-compute-router/scheduler"` | 镜像地址 |
| `scheduler.image.tag` | string | `""` | 镜像标签 |
| `scheduler.image.pullPolicy` | string | `"IfNotPresent"` | 镜像拉取策略 |
| `scheduler.logLevel` | string | `"info"` | 日志级别 |
| `scheduler.leaderElection.enabled` | bool | `true` | 是否启用 Leader 选举 |
| `scheduler.resources.requests.cpu` | string | `"200m"` | CPU 请求 |
| `scheduler.resources.requests.memory` | string | `"256Mi"` | 内存请求 |
| `scheduler.resources.limits.cpu` | string | `"1000m"` | CPU 限制 |
| `scheduler.resources.limits.memory` | string | `"1Gi"` | 内存限制 |
| `scheduler.nodeSelector` | object | `{}` | 节点选择器 |
| `scheduler.tolerations` | list | `[]` | 容忍配置 |
| `scheduler.affinity` | object | `{}` | 亲和性配置 |
| `scheduler.serviceAccount.create` | bool | `true` | 是否创建 ServiceAccount |
| `scheduler.binPacking.enabled` | bool | `true` | 是否启用装箱优化 |

### Scheduler 算力汇率配置

```yaml
scheduler:
  exchangeRates:
    # 以 A100 为基准 (1.0)
    nvidia-a100: 1.0
    nvidia-a800: 0.95
    nvidia-h100: 1.5
    huawei-910b: 0.85
    hygon-dcu: 0.6
```

## Webhook 配置

Webhook 实现 Pod 变更注入，自动配置运行时环境。

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `webhook.enabled` | bool | `true` | 是否启用 Webhook |
| `webhook.replicas` | int | `2` | 副本数（生产环境建议 3） |
| `webhook.image.repository` | string | `"ghcr.io/zrs-io/hetero-compute-router/webhook"` | 镜像地址 |
| `webhook.image.tag` | string | `""` | 镜像标签 |
| `webhook.image.pullPolicy` | string | `"IfNotPresent"` | 镜像拉取策略 |
| `webhook.logLevel` | string | `"info"` | 日志级别 |
| `webhook.failurePolicy` | string | `"Fail"` | Webhook 失败策略 (Fail/Ignore) |
| `webhook.resources.requests.cpu` | string | `"100m"` | CPU 请求 |
| `webhook.resources.requests.memory` | string | `"128Mi"` | 内存请求 |
| `webhook.resources.limits.cpu` | string | `"500m"` | CPU 限制 |
| `webhook.resources.limits.memory` | string | `"512Mi"` | 内存限制 |
| `webhook.nodeSelector` | object | `{}` | 节点选择器 |
| `webhook.tolerations` | list | `[]` | 容忍配置 |
| `webhook.affinity` | object | `{}` | 亲和性配置 |
| `webhook.serviceAccount.create` | bool | `true` | 是否创建 ServiceAccount |

### Webhook TLS 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `webhook.certManager.enabled` | bool | `true` | 是否使用 cert-manager |
| `webhook.tls.caBundle` | string | `""` | 手动配置的 CA 证书（不使用 cert-manager 时） |

## Interceptor 配置

Interceptor 是 API 劫持库，实现软件定义的显存切分。

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `interceptor.enabled` | bool | `true` | 是否启用 Interceptor |
| `interceptor.hostPath` | string | `"/var/lib/hcs"` | 主机上的库文件路径 |

## CRD 配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `crds.install` | bool | `true` | 是否安装 CRD |
| `crds.keep` | bool | `true` | 卸载时是否保留 CRD |

## 网络策略

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `networkPolicy.enabled` | bool | `false` | 是否启用网络策略 |

## 监控配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `serviceMonitor.enabled` | bool | `false` | 是否创建 ServiceMonitor (Prometheus) |
| `serviceMonitor.interval` | string | `"30s"` | 采集间隔 |
| `serviceMonitor.labels` | object | `{}` | ServiceMonitor 额外标签 |

## Pod 安全配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `podSecurityStandards.securityContext` | object | 见下方 | Pod 安全上下文 |

### 默认安全上下文

```yaml
podSecurityStandards:
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
```

## 配置示例

### 开发环境 (values-dev.yaml)

```yaml
global:
  namespace: hcs-dev

nodeAgent:
  logLevel: debug
  reportInterval: 10
  resources:
    requests:
      cpu: 50m
      memory: 64Mi

scheduler:
  replicas: 1
  logLevel: debug
  leaderElection:
    enabled: false

webhook:
  replicas: 1
  logLevel: debug
  failurePolicy: Ignore

serviceMonitor:
  enabled: false

networkPolicy:
  enabled: false
```

### 生产环境 (values-prod.yaml)

```yaml
global:
  namespace: hcs-system

nodeAgent:
  logLevel: info
  reportInterval: 30
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi

scheduler:
  replicas: 3
  logLevel: info
  leaderElection:
    enabled: true
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 2Gi
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 100
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app.kubernetes.io/component: scheduler
            topologyKey: kubernetes.io/hostname

webhook:
  replicas: 3
  logLevel: info
  failurePolicy: Fail
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

serviceMonitor:
  enabled: true
  interval: 30s

networkPolicy:
  enabled: true

podSecurityStandards:
  securityContext:
    runAsNonRoot: true
    seccompProfile:
      type: RuntimeDefault
```

### 自定义镜像仓库

```yaml
global:
  imageRegistry: "registry.example.com"
  imagePullSecrets:
    - name: my-registry-secret

nodeAgent:
  image:
    repository: registry.example.com/hcs/node-agent
    tag: v0.4.0

scheduler:
  image:
    repository: registry.example.com/hcs/scheduler
    tag: v0.4.0

webhook:
  image:
    repository: registry.example.com/hcs/webhook
    tag: v0.4.0
```

## 参数覆盖

使用 `--set` 覆盖单个参数：

```bash
helm install hcs ./chart/hcs \
  --set scheduler.replicas=3 \
  --set webhook.failurePolicy=Ignore
```

使用 `-f` 指定 values 文件：

```bash
helm install hcs ./chart/hcs \
  -f my-values.yaml \
  -f my-overrides.yaml
```

## 下一步

- 查看 [快速开始指南](quick-start.md) 了解安装步骤
- 查看 [前置条件](prerequisites.md) 了解环境要求
