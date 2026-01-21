# 项目清理与整理报告

**更新时间**: 2026-01-21
**项目状态**: 设计阶段 (Design Phase)

---

## 当前项目状态

### 已确认的文件

| 文件/目录 | 状态 | 说明 |
|-----------|------|------|
| `README.md` | ✅ 保留 | 项目总体设计文档，内容完整 |
| `CLAUDE.md` | ✅ 保留 | AI 辅助开发指导文档，已中文化 |
| `docs/` | ✅ 新建 | 项目文档目录 |
| `.git/` | ✅ 保留 | Git 仓库 |
| `.claude/` | ✅ 保留 | Claude Code 配置 |

### 需要建立的目录结构

```
hetero-compute-router/
├── cmd/                    # 主程序入口 (待创建)
│   ├── node-agent/         # Node-Agent 主程序
│   ├── scheduler/          # 调度器插件
│   └── webhook/            # Admission Webhook
├── pkg/                    # 库代码 (待创建)
│   ├── api/                # API 定义
│   ├── detectors/          # 硬件检测器
│   ├── collectors/         # 指标采集器
│   ├── scheduler/          # 调度逻辑
│   └── webhook/            # Webhook 处理
├── config/                 # 配置文件 (待创建)
│   ├── crd/                # CRD 定义
│   ├── rbac/               # RBAC 配置
│   └── webhook/            # Webhook 配置
├── docs/                   # 文档 (已创建)
│   ├── design/             # 设计文档
│   ├── progress/           # 进度文档
│   ├── api/                # API 文档
│   └── archived/           # 归档文档
├── interceptor/            # API 劫持库 (待创建)
│   ├── c/                  # C/C++ 源码
│   └── rust/               # Rust 控制逻辑
├── deploy/                 # 部署文件 (待创建)
│   ├── yaml/               # Kubernetes YAML
│   ├── helm/               # Helm Charts
│   └── docker/             # Dockerfile
├── test/                   # 测试 (待创建)
│   ├── e2e/                # 端到端测试
│   ├── unit/               # 单元测试
│   └── benchmark/          # 性能测试
├── .github/                # GitHub 配置 (待创建)
│   └── workflows/          # CI/CD
├── go.mod                  # Go 模块 (待创建)
├── Makefile                # 构建脚本 (待创建)
└── Dockerfile              # 容器镜像 (待创建)
```

---

## 冗余/过时内容分析

### 结论

**当前项目处于设计阶段，没有冗余或过时的代码、脚本、配置文件。**

原因：
1. 项目刚初始化，只有设计文档
2. 尚未编写任何代码
3. 尚未创建任何配置或脚本
4. Git 仓库只有一个 commit

---

## 需要清理的内容

无 - 项目当前是干净的初始状态。

---

## 建议的下一步

### 立即行动 (P0)

1. **初始化 Go 项目**
   ```bash
   go mod init github.com/zrs-products/hetero-compute-router
   mkdir -p cmd pkg config deploy test
   ```

2. **安装 kubebuilder** (用于 CRD 开发)
   ```bash
   go install sigs.k8s.io/controller-runtime/cmd/setup-envtest@latest
   ```

3. **创建基础 Makefile**
   - make build
   - make test
   - make manifests
   - make deploy

### 短期行动 (P1)

1. **定义 CRD 结构**
   - 创建 `config/crd/bases/` 目录
   - 编写 ComputeNode CRD
   - 编写 ComputeQuota CRD

2. **创建 Node-Agent 框架**
   - cmd/node-agent/main.go
   - pkg/detectors/interface.go
   - pkg/collectors/interface.go

3. **设置 CI/CD**
   - .github/workflows/ci.yml
   - .github/workflows/lint.yml

### 中期行动 (P2)

1. **实现硬件检测器**
   - pkg/detectors/nvidia/nvml.go
   - pkg/detectors/huawei/dsmi.go

2. **创建测试框架**
   - test/e2e/e2e_test.go
   - test/unit/detectors_test.go

3. **编写部署文档**
   - deploy/README.md
   - deploy/local-dev.md

---

## 代码规范建议

### Go 代码规范

```go
// 文件头
// Copyright (c) 2026 ZRS Products

// 包名应为小写单词
package detector

// 导入分组
import (
    "标准库"
    "第三方库"
    "本项目"
)

// 接口定义
type Detector interface {
    Detect() HardwareType
    GetDevices() []Device
}

// 错误处理
if err != nil {
    return fmt.Errorf("failed to detect: %w", err)
}
```

### 提交信息规范

```
<type>(<scope>): <subject>

<body>

<footer>
```

类型:
- feat: 新功能
- fix: 修复
- docs: 文档
- style: 格式
- refactor: 重构
- test: 测试
- chore: 构建/工具

示例:
```
feat(node-agent): add NVIDIA GPU detection

Implement basic GPU detection using NVML library.
- Detect GPU model and VRAM size
- Report via ComputeNode CRD

Closes #1
```

---

## 文档维护规范

### 文档更新时机

| 事件 | 更新文档 |
|------|----------|
| 完成 Phase 里程碑 | docs/progress/current-status.md |
| 修改 API | docs/api/crd-reference.md |
| 架构变更 | docs/design/architecture.md |
| 发布版本 | docs/release-notes/vX.Y.Z.md |
| 废弃功能 | docs/archived/deprecated-features.md |

### 文档审阅

- 每个里程碑结束后审阅所有文档
- API 变更需要更新 CRD 参考
- 设计变更需要同步更新架构文档
