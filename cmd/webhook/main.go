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
	// MetricsAddr is the address the metric endpoint binds to
	MetricsAddr string

	// WebhookPort is the port the webhook server listens on
	WebhookPort int

	// EnableLeaderElection enables leader election
	EnableLeaderElection bool
)

func init() {
	flag.StringVar(&MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
	flag.IntVar(&WebhookPort, "webhook-port", 9443, "The port the webhook server listens on")
	flag.BoolVar(&EnableLeaderElection, "enable-leader-election", false, "Enable leader election")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO: Phase 2 实现 - 初始化 Admission Webhook
	klog.Info("Initializing Hetero Compute Admission Webhook")

	// TODO: Phase 2 实现 - 注册 Mutating Webhook
	klog.Info("Setting up mutating webhook")

	// TODO: Phase 2 实现 - 实现环境变量注入逻辑
	klog.Info("Initializing environment injection")

	// TODO: Phase 3 实现 - 实现动态镜像重定向
	klog.Info("Initializing image redirection")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	klog.Info("Webhook started successfully")

	// Wait for shutdown signal
	<-sigCh
	klog.Info("Shutting down Webhook")

	// TODO: 清理资源

	fmt.Println("Webhook stopped")
}
