package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodMutator handles pod mutation for vendor-specific injection
type PodMutator struct {
	Client      client.Client
	Injector    *Injector
	HCSInjector *HCSInjector
	decoder     admission.Decoder
}

// PodMutatorOption is a function that configures a PodMutator
type PodMutatorOption func(*PodMutator)

// WithPodMutatorInjector sets the vendor injector
func WithPodMutatorInjector(injector *Injector) PodMutatorOption {
	return func(m *PodMutator) {
		m.Injector = injector
	}
}

// WithPodMutatorHCSInjector sets the HCS interceptor injector
func WithPodMutatorHCSInjector(hcsInjector *HCSInjector) PodMutatorOption {
	return func(m *PodMutator) {
		m.HCSInjector = hcsInjector
	}
}

// NewPodMutator creates a new PodMutator with the given client and options
func NewPodMutator(client client.Client, opts ...PodMutatorOption) *PodMutator {
	m := &PodMutator{
		Client:      client,
		Injector:    NewInjector(),
		HCSInjector: NewHCSInjector(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// NewPodMutatorWithInjector creates a new PodMutator with the given client and injector
// Deprecated: Use NewPodMutator with WithPodMutatorInjector option instead
func NewPodMutatorWithInjector(client client.Client, injector *Injector) *PodMutator {
	if injector == nil {
		injector = NewInjector()
	}
	return &PodMutator{
		Client:      client,
		Injector:    injector,
		HCSInjector: NewHCSInjector(),
	}
}

// Handle implements admission.Handler interface
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues(
		"namespace", req.Namespace,
		"name", req.Name,
		"operation", req.Operation,
	)

	pod := &corev1.Pod{}
	if err := m.decoder.Decode(req, pod); err != nil {
		logger.Error(err, "failed to decode pod")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode pod: %w", err))
	}

	// Check if this pod needs injection
	if !NeedsInjection(pod) {
		logger.V(1).Info("pod does not need injection, skipping")
		return admission.Allowed("no injection needed")
	}

	// Determine vendor from various sources
	vendor := m.determineVendor(ctx, pod, req)
	if vendor == "" {
		vendor = m.Injector.defaultVendor
	}

	logger.Info("injecting vendor configuration", "vendor", vendor)

	// Perform injection
	result, err := m.Injector.InjectPod(pod, vendor)
	if err != nil {
		logger.Error(err, "failed to inject pod")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("injection failed: %w", err))
	}

	if result.Injected {
		logger.Info("injection completed",
			"envVarsAdded", result.EnvVarsAdded,
			"volumesAdded", result.VolumesAdded,
		)
	}

	// Perform HCS interceptor injection for VRAM quota enforcement
	if m.HCSInjector != nil && m.HCSInjector.IsEnabled() {
		hcsResult, err := m.HCSInjector.InjectHCS(pod)
		if err != nil {
			logger.Error(err, "failed to inject HCS interceptor")
			// Non-fatal error, continue with vendor injection result
		} else if hcsResult.Injected {
			logger.Info("HCS injection completed",
				"vramQuota", hcsResult.VRAMQuota,
				"containersInjected", hcsResult.ContainersInjected,
			)
		}
	}

	// Marshal the mutated pod
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		logger.Error(err, "failed to marshal pod")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to marshal pod: %w", err))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder
func (m *PodMutator) InjectDecoder(d admission.Decoder) error {
	m.decoder = d
	return nil
}

// determineVendor determines the vendor for a pod based on various sources
func (m *PodMutator) determineVendor(ctx context.Context, pod *corev1.Pod, req admission.Request) VendorType {
	// 1. Check pod annotations first (explicit override)
	if pod.Annotations != nil {
		if vendor, ok := pod.Annotations["hcs.io/vendor"]; ok {
			return VendorType(vendor)
		}
	}

	// 2. Try to detect from resource requests
	if vendor := DetectVendorFromResources(pod); vendor != "" {
		return vendor
	}

	// 3. If pod is already scheduled (has NodeName), try to get node labels
	if pod.Spec.NodeName != "" && m.Client != nil {
		node := &corev1.Node{}
		if err := m.Client.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node); err == nil {
			return m.Injector.detectVendorFromLabels(node.Labels)
		}
	}

	// 4. Fall back to default
	return ""
}

// PodValidator handles pod validation (optional, for future use)
type PodValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// NewPodValidator creates a new PodValidator
func NewPodValidator(client client.Client) *PodValidator {
	return &PodValidator{
		Client: client,
	}
}

// Handle implements admission.Handler interface for validation
func (v *PodValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues(
		"namespace", req.Namespace,
		"name", req.Name,
		"operation", req.Operation,
	)

	pod := &corev1.Pod{}
	if err := v.decoder.Decode(req, pod); err != nil {
		logger.Error(err, "failed to decode pod")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode pod: %w", err))
	}

	// Validate ai.compute resource requests
	if err := v.validateResourceRequests(pod); err != nil {
		logger.Info("pod validation failed", "error", err)
		return admission.Denied(err.Error())
	}

	return admission.Allowed("validation passed")
}

// InjectDecoder injects the decoder
func (v *PodValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// validateResourceRequests validates the resource requests in a pod
func (v *PodValidator) validateResourceRequests(pod *corev1.Pod) error {
	for _, container := range pod.Spec.Containers {
		if err := v.validateContainerResources(&container); err != nil {
			return fmt.Errorf("container %s: %w", container.Name, err)
		}
	}

	for _, container := range pod.Spec.InitContainers {
		if err := v.validateContainerResources(&container); err != nil {
			return fmt.Errorf("init container %s: %w", container.Name, err)
		}
	}

	return nil
}

// validateContainerResources validates a single container's resource requirements
func (v *PodValidator) validateContainerResources(container *corev1.Container) error {
	// Check for conflicting vendor-specific resource requests
	vendorResources := make(map[VendorType]bool)

	checkResource := func(name string) {
		switch {
		case len(name) > 11 && name[:11] == "nvidia.com/":
			vendorResources[VendorNVIDIA] = true
		case len(name) > 11 && name[:11] == "huawei.com/":
			vendorResources[VendorHuawei] = true
		case len(name) > 10 && name[:10] == "hygon.com/":
			vendorResources[VendorHygon] = true
		case len(name) > 14 && name[:14] == "cambricon.com/":
			vendorResources[VendorCambricon] = true
		}
	}

	for resourceName := range container.Resources.Requests {
		checkResource(string(resourceName))
	}
	for resourceName := range container.Resources.Limits {
		checkResource(string(resourceName))
	}

	if len(vendorResources) > 1 {
		return fmt.Errorf("conflicting vendor-specific resources requested")
	}

	return nil
}

// WebhookConfig holds configuration for the webhook server
type WebhookConfig struct {
	// Port is the port the webhook server listens on
	Port int

	// CertDir is the directory containing TLS certificates
	CertDir string

	// CertName is the name of the TLS certificate file
	CertName string

	// KeyName is the name of the TLS key file
	KeyName string

	// MutatePath is the path for the mutating webhook
	MutatePath string

	// ValidatePath is the path for the validating webhook
	ValidatePath string
}

// DefaultWebhookConfig returns the default webhook configuration
func DefaultWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		Port:         9443,
		CertDir:      "/tmp/k8s-webhook-server/serving-certs",
		CertName:     "tls.crt",
		KeyName:      "tls.key",
		MutatePath:   "/mutate-v1-pod",
		ValidatePath: "/validate-v1-pod",
	}
}

// SetupWithManager sets up the webhook with a controller-runtime manager
// This is a helper function for integration with the manager setup
func SetupWebhookWithManager(mgr interface {
	GetClient() client.Client
	GetWebhookServer() interface {
		Register(path string, hook http.Handler)
	}
}, config *WebhookConfig, injector *Injector, hcsInjector *HCSInjector) error {
	if config == nil {
		config = DefaultWebhookConfig()
	}

	c := mgr.GetClient()
	opts := []PodMutatorOption{}
	if injector != nil {
		opts = append(opts, WithPodMutatorInjector(injector))
	}
	if hcsInjector != nil {
		opts = append(opts, WithPodMutatorHCSInjector(hcsInjector))
	}
	mutator := NewPodMutator(c, opts...)

	// Note: In a real setup, you would register with the webhook server
	// This is a simplified version - actual registration depends on controller-runtime version
	_ = mutator

	return nil
}
