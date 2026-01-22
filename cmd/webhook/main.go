package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/webhook"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
}

var (
	// MetricsAddr is the address the metric endpoint binds to
	MetricsAddr string

	// HealthProbeAddr is the address the health probe endpoint binds to
	HealthProbeAddr string

	// WebhookPort is the port the webhook server listens on
	WebhookPort int

	// CertDir is the directory containing TLS certificates
	CertDir string

	// DefaultVendor is the default vendor for injection
	DefaultVendor string

	// EnableLeaderElection enables leader election
	EnableLeaderElection bool
)

func init() {
	flag.StringVar(&MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to")
	flag.StringVar(&HealthProbeAddr, "health-probe-addr", ":8081", "The address the health probe endpoint binds to")
	flag.IntVar(&WebhookPort, "webhook-port", 9443, "The port the webhook server listens on")
	flag.StringVar(&CertDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs", "Directory containing TLS certificates")
	flag.StringVar(&DefaultVendor, "default-vendor", "nvidia", "Default vendor for injection (nvidia, huawei, hygon, cambricon)")
	flag.BoolVar(&EnableLeaderElection, "enable-leader-election", false, "Enable leader election")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	ctrl.SetLogger(klog.NewKlogr())

	klog.Info("Starting Hetero Compute Admission Webhook")
	klog.Infof("Webhook port: %d", WebhookPort)
	klog.Infof("Certificate directory: %s", CertDir)
	klog.Infof("Default vendor: %s", DefaultVendor)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create manager with webhook server
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: MetricsAddr,
		},
		HealthProbeBindAddress: HealthProbeAddr,
		LeaderElection:         EnableLeaderElection,
		LeaderElectionID:       "hcs-webhook-leader",
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port:    WebhookPort,
			CertDir: CertDir,
		}),
	})
	if err != nil {
		klog.Errorf("Failed to create manager: %v", err)
		os.Exit(1)
	}

	// Create injector with options
	injectorOpts := []webhook.InjectorOption{
		webhook.WithDefaultVendor(webhook.VendorType(DefaultVendor)),
	}
	injector := webhook.NewInjector(injectorOpts...)

	// Create HCS injector for VRAM quota enforcement
	hcsInjector := webhook.NewHCSInjector()

	// Create mutator and validator
	mutator := webhook.NewPodMutator(mgr.GetClient(),
		webhook.WithPodMutatorInjector(injector),
		webhook.WithPodMutatorHCSInjector(hcsInjector),
	)
	validator := webhook.NewPodValidator(mgr.GetClient())

	// Get webhook server and register handlers
	webhookServer := mgr.GetWebhookServer()

	// Register mutating webhook
	klog.Info("Registering mutating webhook at /mutate-v1-pod")
	webhookServer.Register("/mutate-v1-pod", &ctrlwebhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
			return mutator.Handle(ctx, req)
		}),
	})

	// Register validating webhook
	klog.Info("Registering validating webhook at /validate-v1-pod")
	webhookServer.Register("/validate-v1-pod", &ctrlwebhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, req admission.Request) admission.Response {
			return validator.Handle(ctx, req)
		}),
	})

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		klog.Errorf("Failed to set up health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		klog.Errorf("Failed to set up ready check: %v", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start manager in goroutine
	go func() {
		klog.Info("Starting webhook manager")
		if err := mgr.Start(ctx); err != nil {
			klog.Errorf("Webhook manager exited with error: %v", err)
			os.Exit(1)
		}
	}()

	klog.Info("Webhook started successfully")
	klog.Infof("Listening on port %d", WebhookPort)

	// Wait for shutdown signal
	sig := <-sigCh
	klog.Infof("Received signal %v, shutting down", sig)

	// Cancel context to trigger graceful shutdown
	cancel()

	klog.Info("Webhook stopped")
}
