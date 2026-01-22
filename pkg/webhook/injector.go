package webhook

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// InjectionResult contains the result of an injection operation
type InjectionResult struct {
	// Injected indicates if any injection was performed
	Injected bool

	// VendorUsed is the vendor profile that was used for injection
	VendorUsed VendorType

	// EnvVarsAdded is the count of environment variables added
	EnvVarsAdded int

	// VolumesAdded is the count of volumes added
	VolumesAdded int

	// Errors contains any non-fatal errors encountered
	Errors []string
}

// Injector handles the injection of vendor-specific configurations into Pods
type Injector struct {
	// defaultVendor is used when vendor cannot be determined
	defaultVendor VendorType

	// skipContainers is a list of container names to skip during injection
	skipContainers map[string]bool

	// customProfiles allows runtime profile customization
	customProfiles map[VendorType]*VendorProfile
}

// InjectorOption is a function that configures an Injector
type InjectorOption func(*Injector)

// WithDefaultVendor sets the default vendor when none is specified
func WithDefaultVendor(vendor VendorType) InjectorOption {
	return func(i *Injector) {
		i.defaultVendor = vendor
	}
}

// WithSkipContainers sets container names to skip during injection
func WithSkipContainers(names ...string) InjectorOption {
	return func(i *Injector) {
		for _, name := range names {
			i.skipContainers[name] = true
		}
	}
}

// WithCustomProfile registers a custom profile for a vendor
func WithCustomProfile(profile *VendorProfile) InjectorOption {
	return func(i *Injector) {
		if profile != nil {
			i.customProfiles[profile.Vendor] = profile
		}
	}
}

// NewInjector creates a new Injector with the given options
func NewInjector(opts ...InjectorOption) *Injector {
	i := &Injector{
		defaultVendor:  VendorNVIDIA,
		skipContainers: make(map[string]bool),
		customProfiles: make(map[VendorType]*VendorProfile),
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

// InjectPod injects vendor-specific configurations into a Pod based on the vendor type.
// It modifies the Pod in place and returns the injection result.
func (i *Injector) InjectPod(pod *corev1.Pod, vendor VendorType) (*InjectionResult, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	result := &InjectionResult{
		VendorUsed: vendor,
	}

	// Get the profile for this vendor
	profile := i.getProfile(vendor)
	if profile == nil {
		return nil, fmt.Errorf("no profile found for vendor: %s", vendor)
	}

	// Inject into all containers
	for idx := range pod.Spec.Containers {
		container := &pod.Spec.Containers[idx]
		if i.skipContainers[container.Name] {
			continue
		}

		envCount := i.injectEnvVars(container, profile)
		mountCount := i.injectVolumeMounts(container, profile)

		result.EnvVarsAdded += envCount
		result.VolumesAdded += mountCount
	}

	// Inject into init containers as well
	for idx := range pod.Spec.InitContainers {
		container := &pod.Spec.InitContainers[idx]
		if i.skipContainers[container.Name] {
			continue
		}

		i.injectEnvVars(container, profile)
		i.injectVolumeMounts(container, profile)
	}

	// Add volumes to pod spec
	volumesAdded := i.injectVolumes(pod, profile)
	result.VolumesAdded += volumesAdded

	// Set runtime class if specified
	if profile.HasRuntimeClass() {
		if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName == "" {
			pod.Spec.RuntimeClassName = &profile.RuntimeClassName
		}
	}

	// Add annotations
	if len(profile.Annotations) > 0 {
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		for k, v := range profile.Annotations {
			if _, exists := pod.Annotations[k]; !exists {
				pod.Annotations[k] = v
			}
		}
	}

	result.Injected = result.EnvVarsAdded > 0 || result.VolumesAdded > 0

	return result, nil
}

// InjectPodByNodeLabels injects configurations based on node labels.
// It extracts the vendor from node labels and delegates to InjectPod.
func (i *Injector) InjectPodByNodeLabels(pod *corev1.Pod, nodeLabels map[string]string) (*InjectionResult, error) {
	vendor := i.detectVendorFromLabels(nodeLabels)
	return i.InjectPod(pod, vendor)
}

// InjectPodByAnnotations injects configurations based on pod annotations.
// Looks for hcs.io/vendor annotation to determine the vendor.
func (i *Injector) InjectPodByAnnotations(pod *corev1.Pod) (*InjectionResult, error) {
	if pod == nil {
		return nil, fmt.Errorf("pod is nil")
	}

	vendor := i.detectVendorFromAnnotations(pod.Annotations)
	return i.InjectPod(pod, vendor)
}

// getProfile returns the profile for a vendor, checking custom profiles first
func (i *Injector) getProfile(vendor VendorType) *VendorProfile {
	// Check custom profiles first
	if profile, ok := i.customProfiles[vendor]; ok {
		return profile
	}
	// Fall back to registry
	return GetProfile(vendor)
}

// detectVendorFromLabels extracts vendor information from node labels
func (i *Injector) detectVendorFromLabels(labels map[string]string) VendorType {
	if labels == nil {
		return i.defaultVendor
	}

	// Check common vendor label patterns
	vendorLabels := []string{
		"hcs.io/vendor",
		"node.kubernetes.io/accelerator-vendor",
		"accelerator/vendor",
	}

	for _, label := range vendorLabels {
		if vendor, ok := labels[label]; ok {
			return VendorType(strings.ToLower(vendor))
		}
	}

	// Check for specific GPU/NPU presence labels
	if _, ok := labels["nvidia.com/gpu.present"]; ok {
		return VendorNVIDIA
	}
	if _, ok := labels["huawei.com/npu"]; ok {
		return VendorHuawei
	}
	if _, ok := labels["hygon.com/dcu"]; ok {
		return VendorHygon
	}
	if _, ok := labels["cambricon.com/mlu"]; ok {
		return VendorCambricon
	}

	return i.defaultVendor
}

// detectVendorFromAnnotations extracts vendor information from pod annotations
func (i *Injector) detectVendorFromAnnotations(annotations map[string]string) VendorType {
	if annotations == nil {
		return i.defaultVendor
	}

	if vendor, ok := annotations["hcs.io/vendor"]; ok {
		return VendorType(strings.ToLower(vendor))
	}

	return i.defaultVendor
}

// injectEnvVars injects environment variables into a container
func (i *Injector) injectEnvVars(container *corev1.Container, profile *VendorProfile) int {
	if container == nil || profile == nil {
		return 0
	}

	// Build a map of existing env vars for quick lookup
	existingEnvs := make(map[string]int)
	for idx, env := range container.Env {
		existingEnvs[env.Name] = idx
	}

	count := 0
	for name, value := range profile.EnvVars {
		if idx, exists := existingEnvs[name]; exists {
			// Special handling for PATH and LD_LIBRARY_PATH - prepend instead of replace
			if name == "PATH" || name == "LD_LIBRARY_PATH" {
				existingValue := container.Env[idx].Value
				if !strings.Contains(existingValue, value) {
					container.Env[idx].Value = value + ":" + existingValue
				}
			}
			// For other vars, don't override existing values
			continue
		}

		container.Env = append(container.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
		count++
	}

	return count
}

// injectVolumeMounts injects volume mounts into a container
func (i *Injector) injectVolumeMounts(container *corev1.Container, profile *VendorProfile) int {
	if container == nil || profile == nil {
		return 0
	}

	// Build a map of existing mounts for quick lookup
	existingMounts := make(map[string]bool)
	for _, mount := range container.VolumeMounts {
		existingMounts[mount.MountPath] = true
	}

	count := 0

	// Add volume mounts from profile
	for _, vc := range profile.Volumes {
		if existingMounts[vc.MountPath] {
			continue
		}
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      vc.Name,
			MountPath: vc.MountPath,
			ReadOnly:  vc.ReadOnly,
		})
		existingMounts[vc.MountPath] = true
		count++
	}

	// Add device mounts
	for idx, dc := range profile.Devices {
		if existingMounts[dc.ContainerPath] {
			continue
		}
		volumeName := fmt.Sprintf("%s-device-%d", profile.Vendor, idx)
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: dc.ContainerPath,
		})
		existingMounts[dc.ContainerPath] = true
		count++
	}

	return count
}

// injectVolumes injects volumes into a pod spec
func (i *Injector) injectVolumes(pod *corev1.Pod, profile *VendorProfile) int {
	if pod == nil || profile == nil {
		return 0
	}

	// Build a map of existing volumes for quick lookup
	existingVolumes := make(map[string]bool)
	for _, vol := range pod.Spec.Volumes {
		existingVolumes[vol.Name] = true
	}

	count := 0

	// Add volumes from profile
	for _, vc := range profile.Volumes {
		if existingVolumes[vc.Name] {
			continue
		}
		hostPathType := corev1.HostPathDirectory
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: vc.Name,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: vc.HostPath,
					Type: &hostPathType,
				},
			},
		})
		existingVolumes[vc.Name] = true
		count++
	}

	// Add device volumes
	for idx, dc := range profile.Devices {
		volumeName := fmt.Sprintf("%s-device-%d", profile.Vendor, idx)
		if existingVolumes[volumeName] {
			continue
		}
		hostPathType := corev1.HostPathCharDev
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: dc.HostPath,
					Type: &hostPathType,
				},
			},
		})
		existingVolumes[volumeName] = true
		count++
	}

	return count
}

// NeedsInjection checks if a pod requires vendor injection based on resource requests
func NeedsInjection(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	// Check all containers for ai.compute resources
	for _, container := range pod.Spec.Containers {
		if hasAIComputeResources(container.Resources) {
			return true
		}
	}

	// Check init containers
	for _, container := range pod.Spec.InitContainers {
		if hasAIComputeResources(container.Resources) {
			return true
		}
	}

	// Check for explicit annotation
	if pod.Annotations != nil {
		if _, ok := pod.Annotations["hcs.io/inject"]; ok {
			return true
		}
	}

	return false
}

// hasAIComputeResources checks if resource requirements contain ai.compute resources
func hasAIComputeResources(resources corev1.ResourceRequirements) bool {
	aiResourcePrefixes := []string{
		"ai.compute/",
		"nvidia.com/gpu",
		"huawei.com/npu",
		"hygon.com/dcu",
		"cambricon.com/mlu",
	}

	// Check requests
	for resourceName := range resources.Requests {
		name := string(resourceName)
		for _, prefix := range aiResourcePrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}

	// Check limits
	for resourceName := range resources.Limits {
		name := string(resourceName)
		for _, prefix := range aiResourcePrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
	}

	return false
}

// DetectVendorFromResources tries to detect the vendor from resource requests
func DetectVendorFromResources(pod *corev1.Pod) VendorType {
	if pod == nil {
		return ""
	}

	vendorPrefixes := map[string]VendorType{
		"nvidia.com/":    VendorNVIDIA,
		"huawei.com/":    VendorHuawei,
		"hygon.com/":     VendorHygon,
		"cambricon.com/": VendorCambricon,
	}

	for _, container := range pod.Spec.Containers {
		for resourceName := range container.Resources.Requests {
			name := string(resourceName)
			for prefix, vendor := range vendorPrefixes {
				if strings.HasPrefix(name, prefix) {
					return vendor
				}
			}
		}
		for resourceName := range container.Resources.Limits {
			name := string(resourceName)
			for prefix, vendor := range vendorPrefixes {
				if strings.HasPrefix(name, prefix) {
					return vendor
				}
			}
		}
	}

	return ""
}
