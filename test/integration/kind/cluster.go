package kind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

const (
	defaultClusterName = "hcs-test"
	kindKubeconfig     = ".kube/kind-config-hcs-test"
)

// TestCluster 管理 kind 测试集群
type TestCluster struct {
	name       string
	kubeconfig string
	provider   *cluster.Provider
	ctx        context.Context
	cancel     context.CancelFunc
}

// New 创建 kind 测试集群
func New(t *testing.T, opts ...Option) *TestCluster {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	tc := &TestCluster{
		name:       defaultClusterName,
		kubeconfig: filepath.Join(os.Getenv("HOME"), kindKubeconfig),
		ctx:        ctx,
		cancel:     cancel,
		provider:   cluster.NewProvider(cluster.ProviderWithDocker()),
	}

	// 应用选项
	for _, opt := range opts {
		opt(tc)
	}

	// 创建集群
	if err := tc.create(); err != nil {
		t.Fatalf("Failed to create kind cluster: %v", err)
	}

	// 等待就绪
	if err := tc.WaitForReady(); err != nil {
		tc.Cleanup()
		t.Fatalf("Cluster not ready: %v", err)
	}

	return tc
}

// create 创建集群
func (tc *TestCluster) create() error {
	kindConfig := &v1alpha4.Cluster{
		Name: tc.name,
		Nodes: []v1alpha4.Node{
			{
				Role:  v1alpha4.ControlPlaneRole,
				Image: "kindest/node:v1.28.0",
			},
			{
				Role:  v1alpha4.WorkerRole,
				Image: "kindest/node:v1.28.0",
				// 挂载 GPU 设备（如果有）
				ExtraMounts: []v1alpha4.Mount{
					{
						HostPath:      "/dev/kfd",
						ContainerPath: "/dev/kfd",
						Readonly:      true,
					},
				},
			},
		},
	}

	return tc.provider.Create(tc.name, cluster.CreateWithV1Alpha4Config(kindConfig))
}

// connect 连接到集群
func (tc *TestCluster) connect() error {
	// 导出 kubeconfig
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", tc.name)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get kubeconfig: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(tc.kubeconfig), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(tc.kubeconfig, out, 0644); err != nil {
		return fmt.Errorf("write kubeconfig: %w", err)
	}

	return nil
}

// Cleanup 清理集群
func (tc *TestCluster) Cleanup() {
	tc.cancel()

	if tc.provider != nil {
		_ = tc.provider.Delete(tc.name, "")
	}
	_ = os.Remove(tc.kubeconfig)
}

// Kubeconfig 返回 kubeconfig 路径
func (tc *TestCluster) Kubeconfig() string {
	return tc.kubeconfig
}

// Name 返回集群名称
func (tc *TestCluster) Name() string {
	return tc.name
}

// WaitForReady 等待集群就绪
func (tc *TestCluster) WaitForReady() error {
	ctx, cancel := context.WithTimeout(tc.ctx, 5*time.Minute)
	defer cancel()

	// 等 kind 命令可用
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for cluster")
		default:
			cmd := exec.Command("kubectl", "cluster-info", "--context", fmt.Sprintf("kind-%s", tc.name))
			if err := cmd.Run(); err == nil {
				// 连接成功
				return nil
			}
			time.Sleep(2 * time.Second)
		}
	}
}

// LoadImage 加载 Docker 镜像到集群
func (tc *TestCluster) LoadImage(image string) error {
	cmd := exec.Command("kind", "load", "docker-image", image,
		"--name", tc.name)
	return cmd.Run()
}

// Kubectl 执行 kubectl 命令
func (tc *TestCluster) Kubectl(args ...string) (string, error) {
	allArgs := append([]string{"--context", fmt.Sprintf("kind-%s", tc.name)}, args...)
	cmd := exec.Command("kubectl", allArgs...)
	out, err := cmd.Output()
	return string(out), err
}

// Option 集群配置选项
type Option func(*TestCluster)

// WithName 设置集群名称
func WithName(name string) Option {
	return func(tc *TestCluster) {
		tc.name = name
	}
}

// WithKubeconfig 设置 kubeconfig 路径
func WithKubeconfig(path string) Option {
	return func(tc *TestCluster) {
		tc.kubeconfig = path
	}
}

// SkipIfExists 如果集群已存在则跳过创建
func SkipIfExists() Option {
	return func(tc *TestCluster) {
		// 检查集群是否存在
		cmd := exec.Command("kind", "get", "clusters")
		if err := cmd.Run(); err == nil {
			// 集群存在，标记为已初始化
			tc.ctx = context.Background()
			tc.cancel = func() {}
		}
	}
}

// IsInstalled 检查 kind 是否安装
func IsInstalled() bool {
	cmd := exec.Command("kind", "version")
	return cmd.Run() == nil
}

// CreateIfNotExists 创建集群（如果不存在）
func CreateIfNotExists(name string) error {
	if !IsInstalled() {
		return fmt.Errorf("kind is not installed")
	}

	// 检查是否已存在
	cmd := exec.Command("kind", "get", "clusters")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("check clusters: %w", err)
	}

	// 检查名称是否在列表中
	if contains(string(out), name) {
		return nil // 已存在
	}

	// 创建新集群
	provider := cluster.NewProvider(cluster.ProviderWithDocker())
	return provider.Create(name, cluster.CreateWithV1Alpha4Config(&v1alpha4.Cluster{
		Name: name,
		Nodes: []v1alpha4.Node{
			{
				Role:  v1alpha4.ControlPlaneRole,
				Image: "kindest/node:v1.28.0",
			},
		},
	}))
}

// DeleteCluster 删除集群
func DeleteCluster(name string) error {
	if !IsInstalled() {
		return nil
	}

	provider := cluster.NewProvider(cluster.ProviderWithDocker())
	return provider.Delete(name, "")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
