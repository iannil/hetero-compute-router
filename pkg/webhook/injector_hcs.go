package webhook

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// HCS resource name for VRAM quota
const (
	HCSVRAMResource     = "ai.compute/vram"
	HCSVRAMQuotaEnvVar  = "HCS_VRAM_QUOTA"
	LDPreloadEnvVar     = "LD_PRELOAD"
	HCSLibVolumeName    = "hcs-lib"
	HCSInjectAnnotation = "hcs.io/vram-inject"
)

// HCSInjectionResult contains the result of HCS injection operation
type HCSInjectionResult struct {
	// Injected indicates if any injection was performed
	Injected bool

	// VRAMQuota is the extracted VRAM quota value
	VRAMQuota string

	// ContainersInjected is the count of containers that were injected
	ContainersInjected int

	// Errors contains any non-fatal errors encountered
	Errors []string
}

// HCSInjector handles the injection of HCS interceptor into Pods
type HCSInjector struct {
	// interceptorPath is the path to libhcs_interceptor.so inside the container
	interceptorPath string

	// hostLibPath is the path on the host where HCS libraries are stored
	hostLibPath string

	// containerLibPath is the path inside the container where HCS libraries are mounted
	containerLibPath string

	// skipContainers is a list of container names to skip during injection
	skipContainers map[string]bool

	// enabled controls whether HCS injection is active
	enabled bool
}

// HCSInjectorOption is a function that configures an HCSInjector
type HCSInjectorOption func(*HCSInjector)

// WithInterceptorPath sets the path to the interceptor library
func WithInterceptorPath(path string) HCSInjectorOption {
	return func(h *HCSInjector) {
		h.interceptorPath = path
	}
}

// WithHostLibPath sets the host path for HCS libraries
func WithHostLibPath(path string) HCSInjectorOption {
	return func(h *HCSInjector) {
		h.hostLibPath = path
	}
}

// WithContainerLibPath sets the container mount path for HCS libraries
func WithContainerLibPath(path string) HCSInjectorOption {
	return func(h *HCSInjector) {
		h.containerLibPath = path
	}
}

// WithHCSSkipContainers sets container names to skip during HCS injection
func WithHCSSkipContainers(names ...string) HCSInjectorOption {
	return func(h *HCSInjector) {
		for _, name := range names {
			h.skipContainers[name] = true
		}
	}
}

// WithHCSEnabled enables or disables HCS injection
func WithHCSEnabled(enabled bool) HCSInjectorOption {
	return func(h *HCSInjector) {
		h.enabled = enabled
	}
}

// NewHCSInjector creates a new HCSInjector with the given options
func NewHCSInjector(opts ...HCSInjectorOption) *HCSInjector {
	h := &HCSInjector{
		interceptorPath:  "/usr/local/hcs/lib/libhcs_interceptor.so",
		hostLibPath:      "/usr/local/hcs/lib",
		containerLibPath: "/usr/local/hcs/lib",
		skipContainers:   make(map[string]bool),
		enabled:          true,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// InjectHCS injects HCS interceptor into a Pod based on VRAM resource requests.
// It modifies the Pod in place and returns the injection result.
func (h *HCSInjector) InjectHCS(pod *corev1.Pod) (*HCSInjectionResult, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	result := &HCSInjectionResult{}

	// Check if HCS injection is enabled
	if !h.enabled {
		return result, nil
	}

	// Check if pod needs HCS injection
	if !NeedsHCSInjection(pod) {
		return result, nil
	}

	// Check for explicit disable annotation
	if pod.Annotations != nil {
		if val, ok := pod.Annotations[HCSInjectAnnotation]; ok && val == "false" {
			return result, nil
		}
	}

	// Extract VRAM quota from pod
	vramQuota := h.extractVRAMQuota(pod)
	if vramQuota == "" {
		return result, nil
	}
	result.VRAMQuota = vramQuota

	// Inject into all containers
	for idx := range pod.Spec.Containers {
		container := &pod.Spec.Containers[idx]
		if h.skipContainers[container.Name] {
			continue
		}

		h.injectContainer(container, vramQuota)
		result.ContainersInjected++
	}

	// Inject into init containers as well
	for idx := range pod.Spec.InitContainers {
		container := &pod.Spec.InitContainers[idx]
		if h.skipContainers[container.Name] {
			continue
		}

		h.injectContainer(container, vramQuota)
	}

	// Add volume to pod spec
	h.injectVolume(pod)

	result.Injected = result.ContainersInjected > 0

	// Add annotation to indicate injection was performed
	if result.Injected {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[HCSInjectAnnotation] = "true"
		pod.Annotations["hcs.io/vram-quota"] = vramQuota
	}

	return result, nil
}

// extractVRAMQuota extracts the VRAM quota from pod resource requests.
// It returns the quota as a string suitable for HCS_VRAM_QUOTA env var.
func (h *HCSInjector) extractVRAMQuota(pod *corev1.Pod) string {
	// Check annotation override first
	if pod.Annotations != nil {
		if quota, ok := pod.Annotations["hcs.io/vram-quota"]; ok && quota != "" {
			return quota
		}
	}

	// Find the maximum VRAM request across all containers
	var maxVRAM *resource.Quantity

	for _, container := range pod.Spec.Containers {
		if quota := h.getContainerVRAMRequest(container); quota != nil {
			if maxVRAM == nil || quota.Cmp(*maxVRAM) > 0 {
				maxVRAM = quota
			}
		}
	}

	if maxVRAM == nil {
		return ""
	}

	// Convert to string format (e.g., "16Gi")
	return maxVRAM.String()
}

// getContainerVRAMRequest returns the VRAM request for a container
func (h *HCSInjector) getContainerVRAMRequest(container corev1.Container) *resource.Quantity {
	// Check requests first
	if container.Resources.Requests != nil {
		if vram, ok := container.Resources.Requests[corev1.ResourceName(HCSVRAMResource)]; ok {
			return &vram
		}
	}

	// Fall back to limits
	if container.Resources.Limits != nil {
		if vram, ok := container.Resources.Limits[corev1.ResourceName(HCSVRAMResource)]; ok {
			return &vram
		}
	}

	return nil
}

// injectContainer injects HCS environment variables and volume mounts into a container
func (h *HCSInjector) injectContainer(container *corev1.Container, vramQuota string) {
	// Inject LD_PRELOAD
	h.injectLDPreload(container)

	// Inject HCS_VRAM_QUOTA
	h.injectVRAMQuota(container, vramQuota)

	// Inject volume mount
	h.injectVolumeMount(container)
}

// injectLDPreload injects or appends to LD_PRELOAD environment variable
func (h *HCSInjector) injectLDPreload(container *corev1.Container) {
	// Find existing LD_PRELOAD
	for idx, env := range container.Env {
		if env.Name == LDPreloadEnvVar {
			// Append our interceptor if not already present
			if !strings.Contains(env.Value, h.interceptorPath) {
				container.Env[idx].Value = h.interceptorPath + ":" + env.Value
			}
			return
		}
	}

	// Add new LD_PRELOAD
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  LDPreloadEnvVar,
		Value: h.interceptorPath,
	})
}

// injectVRAMQuota injects HCS_VRAM_QUOTA environment variable
func (h *HCSInjector) injectVRAMQuota(container *corev1.Container, vramQuota string) {
	// Check if already exists
	for idx, env := range container.Env {
		if env.Name == HCSVRAMQuotaEnvVar {
			// Update existing value
			container.Env[idx].Value = vramQuota
			return
		}
	}

	// Add new env var
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  HCSVRAMQuotaEnvVar,
		Value: vramQuota,
	})
}

// injectVolumeMount injects the HCS library volume mount into a container
func (h *HCSInjector) injectVolumeMount(container *corev1.Container) {
	// Check if already exists
	for _, mount := range container.VolumeMounts {
		if mount.Name == HCSLibVolumeName || mount.MountPath == h.containerLibPath {
			return
		}
	}

	// Add volume mount
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      HCSLibVolumeName,
		MountPath: h.containerLibPath,
		ReadOnly:  true,
	})
}

// injectVolume adds the HCS library volume to the pod spec
func (h *HCSInjector) injectVolume(pod *corev1.Pod) {
	// Check if already exists
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == HCSLibVolumeName {
			return
		}
	}

	// Add volume
	hostPathType := corev1.HostPathDirectory
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: HCSLibVolumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: h.hostLibPath,
				Type: &hostPathType,
			},
		},
	})
}

// NeedsHCSInjection checks if a pod requires HCS interceptor injection based on resource requests
func NeedsHCSInjection(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	// Check for explicit annotation
	if pod.Annotations != nil {
		if val, ok := pod.Annotations[HCSInjectAnnotation]; ok {
			return val == "true"
		}
	}

	// Check all containers for ai.compute/vram resources
	for _, container := range pod.Spec.Containers {
		if hasVRAMResource(container.Resources) {
			return true
		}
	}

	// Check init containers
	for _, container := range pod.Spec.InitContainers {
		if hasVRAMResource(container.Resources) {
			return true
		}
	}

	return false
}

// hasVRAMResource checks if resource requirements contain ai.compute/vram
func hasVRAMResource(resources corev1.ResourceRequirements) bool {
	// Check requests
	if resources.Requests != nil {
		if _, ok := resources.Requests[corev1.ResourceName(HCSVRAMResource)]; ok {
			return true
		}
	}

	// Check limits
	if resources.Limits != nil {
		if _, ok := resources.Limits[corev1.ResourceName(HCSVRAMResource)]; ok {
			return true
		}
	}

	return false
}

// GetInterceptorPath returns the interceptor library path
func (h *HCSInjector) GetInterceptorPath() string {
	return h.interceptorPath
}

// GetHostLibPath returns the host library path
func (h *HCSInjector) GetHostLibPath() string {
	return h.hostLibPath
}

// GetContainerLibPath returns the container library path
func (h *HCSInjector) GetContainerLibPath() string {
	return h.containerLibPath
}

// IsEnabled returns whether HCS injection is enabled
func (h *HCSInjector) IsEnabled() bool {
	return h.enabled
}
