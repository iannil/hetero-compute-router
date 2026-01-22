package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/agent"
	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

var (
	// NodeName is the name of the Kubernetes node
	NodeName string

	// MetricsAddr is the address the metric endpoint binds to
	MetricsAddr string

	// Kubeconfig path for out-of-cluster usage
	Kubeconfig string

	// UseMock enables mock mode for testing without real hardware
	UseMock bool

	// MockDeviceCount number of mock devices
	MockDeviceCount int

	// CollectInterval collection interval
	CollectInterval time.Duration

	// ReportInterval report interval
	ReportInterval time.Duration
)

func init() {
	flag.StringVar(&NodeName, "node-name", os.Getenv("NODE_NAME"), "Name of the Kubernetes node")
	flag.StringVar(&MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
	flag.StringVar(&Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file (for out-of-cluster usage)")
	flag.BoolVar(&UseMock, "mock", false, "Use mock detector for testing")
	flag.IntVar(&MockDeviceCount, "mock-devices", 4, "Number of mock devices")
	flag.DurationVar(&CollectInterval, "collect-interval", 10*time.Second, "Metrics collection interval")
	flag.DurationVar(&ReportInterval, "report-interval", 30*time.Second, "CRD report interval")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if NodeName == "" {
		klog.Error("NODE_NAME must be set via --node-name flag or NODE_NAME environment variable")
		os.Exit(1)
	}

	klog.Infof("Starting Node-Agent for node: %s", NodeName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建 Kubernetes 客户端
	k8sClient, err := createK8sClient()
	if err != nil {
		klog.Errorf("Failed to create Kubernetes client: %v", err)
		klog.Info("Running in standalone mode without CRD reporting")
	}

	// 创建 Reporter（如果有 K8s 客户端）
	var reporter *agent.Reporter
	if k8sClient != nil {
		reporter = agent.NewReporter(k8sClient)
	}

	// 创建 Agent 配置
	config := &agent.Config{
		NodeName:        NodeName,
		CollectInterval: CollectInterval,
		ReportInterval:  ReportInterval,
		UseMock:         UseMock,
	}

	if UseMock {
		config.MockConfig = &agent.MockConfig{
			DeviceCount: MockDeviceCount,
			GPUModel:    "NVIDIA A100-SXM4-80GB",
			VRAMPerGPU:  80 * 1024 * 1024 * 1024,
			HasNVLink:   true,
		}
		klog.Info("Running in mock mode")
	}

	// 创建并启动 Agent
	nodeAgent, err := agent.New(config, reporter)
	if err != nil {
		klog.Errorf("Failed to create agent: %v", err)
		os.Exit(1)
	}

	if err := nodeAgent.Start(ctx); err != nil {
		klog.Errorf("Failed to start agent: %v", err)
		os.Exit(1)
	}

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.Info("Node-Agent is running. Press Ctrl+C to stop.")

	// 等待停止信号
	<-sigCh
	klog.Info("Received shutdown signal")

	// 优雅关闭
	cancel()
	if err := nodeAgent.Stop(); err != nil {
		klog.Errorf("Error during shutdown: %v", err)
	}

	klog.Info("Node-Agent stopped")
}

// createK8sClient 创建 Kubernetes 客户端
func createK8sClient() (client.Client, error) {
	var config *rest.Config
	var err error

	if Kubeconfig != "" {
		// 使用 kubeconfig 文件（集群外运行）
		config, err = clientcmd.BuildConfigFromFlags("", Kubeconfig)
		if err != nil {
			return nil, err
		}
		klog.Info("Using kubeconfig for cluster access")
	} else {
		// 尝试 in-cluster 配置
		config, err = rest.InClusterConfig()
		if err != nil {
			// 尝试默认 kubeconfig 路径
			home, _ := os.UserHomeDir()
			defaultKubeconfig := home + "/.kube/config"
			if _, statErr := os.Stat(defaultKubeconfig); statErr == nil {
				config, err = clientcmd.BuildConfigFromFlags("", defaultKubeconfig)
				if err != nil {
					return nil, err
				}
				klog.Info("Using default kubeconfig")
			} else {
				return nil, err
			}
		} else {
			klog.Info("Using in-cluster config")
		}
	}

	// 创建 controller-runtime client
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func init() {
	ctrl.SetLogger(klog.NewKlogr())
}
