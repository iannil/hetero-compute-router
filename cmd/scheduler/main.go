package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/framework"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/plugins"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

var (
	// Kubeconfig path for out-of-cluster usage
	Kubeconfig string

	// BindAddr is the address to bind extender HTTP server
	BindAddr string

	// MetricsAddr is the address the metric endpoint binds to
	MetricsAddr string
)

func init() {
	flag.StringVar(&Kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file")
	flag.StringVar(&BindAddr, "bind-addr", ":8888", "Address to bind extender HTTP server")
	flag.StringVar(&MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
}

// SchedulerExtender HCS 调度器扩展器
type SchedulerExtender struct {
	client        client.Client
	filterPlugin  *plugins.FilterPlugin
	scorePlugin   *plugins.ScorePlugin
	reservePlugin *plugins.ReservePlugin
}

// ExtenderArgs 调度器扩展请求参数
type ExtenderArgs struct {
	Pod       *v1.Pod      `json:"pod"`
	Nodes     *v1.NodeList `json:"nodes,omitempty"`
	NodeNames *[]string    `json:"nodenames,omitempty"`
}

// ExtenderFilterResult 过滤结果
type ExtenderFilterResult struct {
	Nodes       *v1.NodeList      `json:"nodes,omitempty"`
	NodeNames   *[]string         `json:"nodenames,omitempty"`
	FailedNodes map[string]string `json:"failedNodes,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// HostPriority 节点优先级
type HostPriority struct {
	Host  string `json:"host"`
	Score int64  `json:"score"`
}

// HostPriorityList 节点优先级列表
type HostPriorityList []HostPriority

// NewSchedulerExtender 创建调度器扩展器
func NewSchedulerExtender(c client.Client) *SchedulerExtender {
	return &SchedulerExtender{
		client:        c,
		filterPlugin:  plugins.NewFilterPlugin(c),
		scorePlugin:   plugins.NewScorePlugin(c, nil),
		reservePlugin: plugins.NewReservePlugin(c),
	}
}

// Filter 过滤处理
func (e *SchedulerExtender) Filter(args ExtenderArgs) (*ExtenderFilterResult, error) {
	ctx := context.Background()
	state := framework.NewCycleState()

	// 预过滤
	status := e.filterPlugin.PreFilter(ctx, state, args.Pod)
	if !status.IsSuccess() {
		return &ExtenderFilterResult{
			Error: status.Message,
		}, nil
	}

	result := &ExtenderFilterResult{
		FailedNodes: make(map[string]string),
	}

	var filteredNodes []v1.Node
	var filteredNodeNames []string

	// 从 Nodes 或 NodeNames 获取节点列表
	if args.Nodes != nil {
		for _, node := range args.Nodes.Items {
			nodeInfo := framework.NewNodeInfo(&node)
			status := e.filterPlugin.Filter(ctx, state, args.Pod, nodeInfo)
			if status.IsSuccess() {
				filteredNodes = append(filteredNodes, node)
			} else {
				result.FailedNodes[node.Name] = status.Message
			}
		}
		result.Nodes = &v1.NodeList{Items: filteredNodes}
	} else if args.NodeNames != nil {
		for _, nodeName := range *args.NodeNames {
			node := &v1.Node{}
			node.Name = nodeName
			nodeInfo := framework.NewNodeInfo(node)
			status := e.filterPlugin.Filter(ctx, state, args.Pod, nodeInfo)
			if status.IsSuccess() {
				filteredNodeNames = append(filteredNodeNames, nodeName)
			} else {
				result.FailedNodes[nodeName] = status.Message
			}
		}
		result.NodeNames = &filteredNodeNames
	}

	return result, nil
}

// Prioritize 打分处理
func (e *SchedulerExtender) Prioritize(args ExtenderArgs) (HostPriorityList, error) {
	ctx := context.Background()
	state := framework.NewCycleState()

	// 预过滤以获取请求数据
	e.filterPlugin.PreFilter(ctx, state, args.Pod)

	var priorities HostPriorityList

	// 从 Nodes 或 NodeNames 获取节点列表并打分
	if args.Nodes != nil {
		for _, node := range args.Nodes.Items {
			score, _ := e.scorePlugin.Score(ctx, state, args.Pod, node.Name)
			priorities = append(priorities, HostPriority{
				Host:  node.Name,
				Score: score,
			})
		}
	} else if args.NodeNames != nil {
		for _, nodeName := range *args.NodeNames {
			score, _ := e.scorePlugin.Score(ctx, state, args.Pod, nodeName)
			priorities = append(priorities, HostPriority{
				Host:  nodeName,
				Score: score,
			})
		}
	}

	// 归一化分数
	if len(priorities) > 0 {
		nodeScores := make(framework.NodeScoreList, len(priorities))
		for i, p := range priorities {
			nodeScores[i] = framework.NodeScore{Name: p.Host, Score: p.Score}
		}
		e.scorePlugin.NormalizeScore(ctx, state, args.Pod, nodeScores)
		for i := range priorities {
			priorities[i].Score = nodeScores[i].Score
		}
	}

	return priorities, nil
}

// Bind 绑定处理（预留资源）
func (e *SchedulerExtender) Bind(args ExtenderArgs, nodeName string) error {
	ctx := context.Background()
	state := framework.NewCycleState()

	// 预过滤以获取请求数据
	e.filterPlugin.PreFilter(ctx, state, args.Pod)

	// 预留资源
	status := e.reservePlugin.Reserve(ctx, state, args.Pod, nodeName)
	if !status.IsSuccess() {
		return fmt.Errorf("failed to reserve: %s", status.Message)
	}

	return nil
}

// ServeHTTP HTTP 处理
func (e *SchedulerExtender) ServeHTTP() http.Handler {
	mux := http.NewServeMux()

	// 过滤接口
	mux.HandleFunc("/filter", func(w http.ResponseWriter, r *http.Request) {
		var args ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result, err := e.Filter(args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// 打分接口
	mux.HandleFunc("/prioritize", func(w http.ResponseWriter, r *http.Request) {
		var args ExtenderArgs
		if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		priorities, err := e.Prioritize(args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(priorities)
	})

	// 健康检查
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return mux
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	klog.Info("Initializing Hetero Compute Scheduler Extender")

	// 创建 K8s 客户端
	k8sClient, err := createK8sClient()
	if err != nil {
		klog.Errorf("Failed to create Kubernetes client: %v", err)
		os.Exit(1)
	}

	// 创建调度器扩展器
	extender := NewSchedulerExtender(k8sClient)

	// 启动 HTTP 服务器
	server := &http.Server{
		Addr:    BindAddr,
		Handler: extender.ServeHTTP(),
	}

	// 在 goroutine 中启动服务器
	go func() {
		klog.Infof("Starting scheduler extender HTTP server on %s", BindAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.Errorf("HTTP server error: %v", err)
		}
	}()

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.Info("Scheduler extender started successfully")

	// 等待停止信号
	<-sigCh
	klog.Info("Shutting down Scheduler Extender")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		klog.Errorf("HTTP server shutdown error: %v", err)
	}

	klog.Info("Scheduler extender stopped")
}

// createK8sClient 创建 Kubernetes 客户端
func createK8sClient() (client.Client, error) {
	var config *rest.Config
	var err error

	if Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", Kubeconfig)
		if err != nil {
			return nil, err
		}
		klog.Info("Using kubeconfig for cluster access")
	} else {
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

	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return c, nil
}
