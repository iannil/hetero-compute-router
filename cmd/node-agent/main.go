package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"
)

var (
	// NodeName is the name of the Kubernetes node
	NodeName string

	// MetricsAddr is the address the metric endpoint binds to
	MetricsAddr string

	// EnableLeaderElection enables leader election
	EnableLeaderElection bool
)

func init() {
	flag.StringVar(&NodeName, "node-name", os.Getenv("NODE_NAME"), "Name of the Kubernetes node")
	flag.StringVar(&MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
	flag.BoolVar(&EnableLeaderElection, "enable-leader-election", false, "Enable leader election")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if NodeName == "" {
		klog.Error("NODE_NAME must be set")
		os.Exit(1)
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: Phase 1 实现 - 初始化检测器
	klog.Info("Initializing Node-Agent for node: ", NodeName)

	// TODO: Phase 1 实现 - 启动硬件检测循环
	klog.Info("Starting hardware detection")

	// TODO: Phase 1 实现 - 启动指标采集
	klog.Info("Starting metrics collection")

	// TODO: Phase 1 实现 - 连接 Kubernetes API 并上报状态
	klog.Info("Connecting to Kubernetes API")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.Info("Node-Agent started successfully")

	// Wait for shutdown signal
	<-sigCh
	klog.Info("Shutting down Node-Agent")

	// TODO: 清理资源

	fmt.Println("Node-Agent stopped")
}
