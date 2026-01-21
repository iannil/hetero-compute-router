# 文档目录

本目录包含 Hetero-Compute-Router (HCS) 项目的完整文档。

---

## 目录结构

```
docs/
├── README.md              # 本文件
├── design/                # 设计文档
│   └── architecture.md    # 技术架构设计
├── progress/              # 进度文档
│   ├── current-status.md  # 当前项目状态
│   └── roadmap.md         # 项目路线图
├── api/                   # API 文档
│   └── crd-reference.md   # CRD 和 API 参考
└── archived/              # 归档文档
    └── cleanup-report.md  # 清理与整理报告
```

---

## 文档导航

### 快速开始

| 想要了解 | 阅读 |
|----------|------|
| 项目概述 | [../README.md](../README.md) |
| 当前进度 | [progress/current-status.md](progress/current-status.md) |
| 开发路线图 | [progress/roadmap.md](progress/roadmap.md) |

### 设计文档

| 想要了解 | 阅读 |
|----------|------|
| 总体架构 | [design/architecture.md](design/architecture.md) |
| 三层解耦模型 | [design/architecture.md#1-总体架构) |
| 组件设计 | [design/architecture.md#2-核心组件设计) |
| CRD 设计 | [design/architecture.md#3-crd-设计) |

### API 文档

| 想要了解 | 阅读 |
|----------|------|
| CRD 参考 | [api/crd-reference.md](api/crd-reference.md) |
| ComputeNode | [api/crd-reference.md#computenode) |
| ComputeQuota | [api/crd-reference.md#computequota) |
| Pod 资源请求 | [api/crd-reference.md#pod-资源请求) |

### 历史记录

| 想要了解 | 阅读 |
|----------|------|
| 项目清理记录 | [archived/cleanup-report.md](archived/cleanup-report.md) |

---

## 文档贡献

### 更新文档

当项目发生变化时，请按以下规则更新文档：

| 变更类型 | 需要更新的文档 |
|----------|----------------|
| 完成里程碑 | `progress/current-status.md` |
| API 变更 | `api/crd-reference.md` |
| 架构变更 | `design/architecture.md` |
| 发布版本 | 新建 `api/release-notes/vX.Y.Z.md` |

### 文档规范

- 使用 Markdown 格式
- 标题层级清晰（最多 4 级）
- 代码块指定语言
- 表格用于结构化数据
- 图表使用 ASCII 艺术或 Mermaid

---

## 联系方式

项目相关问题请参考主 [README.md](../README.md)。
