package webhook

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// =============================================================================
// Profile Tests
// =============================================================================

func TestGetProfile(t *testing.T) {
	tests := []struct {
		vendor   VendorType
		expected bool
	}{
		{VendorNVIDIA, true},
		{VendorHuawei, true},
		{VendorHygon, true},
		{VendorCambricon, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		profile := GetProfile(tt.vendor)
		if (profile != nil) != tt.expected {
			t.Errorf("GetProfile(%s) = %v, want profile present: %v", tt.vendor, profile != nil, tt.expected)
		}
	}
}

func TestGetProfileByString(t *testing.T) {
	profile := GetProfileByString("nvidia")
	if profile == nil {
		t.Fatal("GetProfileByString(nvidia) returned nil")
	}
	if profile.Vendor != VendorNVIDIA {
		t.Errorf("Profile vendor = %q, want nvidia", profile.Vendor)
	}
}

func TestListProfiles(t *testing.T) {
	profiles := ListProfiles()
	if len(profiles) != 4 {
		t.Errorf("ListProfiles() returned %d profiles, want 4", len(profiles))
	}
}

func TestListVendors(t *testing.T) {
	vendors := ListVendors()
	if len(vendors) != 4 {
		t.Errorf("ListVendors() returned %d vendors, want 4", len(vendors))
	}

	vendorMap := make(map[VendorType]bool)
	for _, v := range vendors {
		vendorMap[v] = true
	}

	expected := []VendorType{VendorNVIDIA, VendorHuawei, VendorHygon, VendorCambricon}
	for _, v := range expected {
		if !vendorMap[v] {
			t.Errorf("Vendor %s not found in ListVendors()", v)
		}
	}
}

func TestRegisterProfile(t *testing.T) {
	// Create a custom profile
	custom := &VendorProfile{
		Vendor: "custom-vendor",
		EnvVars: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	RegisterProfile(custom)

	// Verify it was registered
	profile := GetProfile("custom-vendor")
	if profile == nil {
		t.Fatal("Custom profile not found after registration")
	}
	if profile.EnvVars["CUSTOM_VAR"] != "value" {
		t.Error("Custom profile env var not set correctly")
	}

	// Clean up
	delete(profileRegistry, "custom-vendor")
}

func TestRegisterProfile_Nil(t *testing.T) {
	// Should not panic
	RegisterProfile(nil)
}

func TestVendorProfile_BuildEnvVars(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	envVars := profile.BuildEnvVars()
	if len(envVars) == 0 {
		t.Error("BuildEnvVars() returned empty slice")
	}

	// Check for expected env vars
	found := false
	for _, env := range envVars {
		if env.Name == "NVIDIA_VISIBLE_DEVICES" {
			found = true
			if env.Value != "all" {
				t.Errorf("NVIDIA_VISIBLE_DEVICES = %q, want all", env.Value)
			}
		}
	}
	if !found {
		t.Error("NVIDIA_VISIBLE_DEVICES not found in env vars")
	}
}

func TestVendorProfile_BuildVolumes(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	volumes := profile.BuildVolumes()
	if len(volumes) != len(profile.Volumes) {
		t.Errorf("BuildVolumes() returned %d volumes, want %d", len(volumes), len(profile.Volumes))
	}

	// Check volume properties
	for _, vol := range volumes {
		if vol.HostPath == nil {
			t.Error("Volume should have HostPath source")
		}
	}
}

func TestVendorProfile_BuildVolumeMounts(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	mounts := profile.BuildVolumeMounts()
	if len(mounts) != len(profile.Volumes) {
		t.Errorf("BuildVolumeMounts() returned %d mounts, want %d", len(mounts), len(profile.Volumes))
	}
}

func TestVendorProfile_BuildDeviceVolumes(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	volumes := profile.BuildDeviceVolumes()
	if len(volumes) != len(profile.Devices) {
		t.Errorf("BuildDeviceVolumes() returned %d volumes, want %d", len(volumes), len(profile.Devices))
	}

	// Check device volume properties
	for _, vol := range volumes {
		if vol.HostPath == nil {
			t.Error("Device volume should have HostPath source")
		}
		if vol.HostPath.Type == nil || *vol.HostPath.Type != corev1.HostPathCharDev {
			t.Error("Device volume should have HostPathCharDev type")
		}
	}
}

func TestVendorProfile_BuildDeviceVolumeMounts(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	mounts := profile.BuildDeviceVolumeMounts()
	if len(mounts) != len(profile.Devices) {
		t.Errorf("BuildDeviceVolumeMounts() returned %d mounts, want %d", len(mounts), len(profile.Devices))
	}
}

func TestVendorProfile_HasRuntimeClass(t *testing.T) {
	tests := []struct {
		vendor   VendorType
		expected bool
	}{
		{VendorNVIDIA, true},     // NVIDIA has runtime class
		{VendorHuawei, false},    // Huawei does not
		{VendorHygon, false},     // Hygon does not
		{VendorCambricon, false}, // Cambricon does not
	}

	for _, tt := range tests {
		profile := GetProfile(tt.vendor)
		if profile == nil {
			t.Fatalf("Profile for %s not found", tt.vendor)
		}
		if profile.HasRuntimeClass() != tt.expected {
			t.Errorf("HasRuntimeClass() for %s = %v, want %v", tt.vendor, profile.HasRuntimeClass(), tt.expected)
		}
	}
}

func TestVendorProfile_Clone(t *testing.T) {
	profile := GetProfile(VendorNVIDIA)
	if profile == nil {
		t.Fatal("NVIDIA profile not found")
	}

	clone := profile.Clone()
	if clone == nil {
		t.Fatal("Clone() returned nil")
	}

	// Verify it's a deep copy
	if clone == profile {
		t.Error("Clone() returned same pointer")
	}

	// Modify clone and verify original is unchanged
	clone.EnvVars["NEW_VAR"] = "test"
	if _, ok := profile.EnvVars["NEW_VAR"]; ok {
		t.Error("Modifying clone affected original")
	}
}

func TestVendorProfile_Clone_Nil(t *testing.T) {
	var profile *VendorProfile
	clone := profile.Clone()
	if clone != nil {
		t.Error("Clone() of nil should return nil")
	}
}

// =============================================================================
// Injector Tests
// =============================================================================

func TestNewInjector(t *testing.T) {
	injector := NewInjector()
	if injector == nil {
		t.Fatal("NewInjector() returned nil")
	}

	if injector.defaultVendor != VendorNVIDIA {
		t.Errorf("Default vendor = %q, want nvidia", injector.defaultVendor)
	}
}

func TestNewInjector_WithOptions(t *testing.T) {
	customProfile := &VendorProfile{
		Vendor: VendorHuawei,
		EnvVars: map[string]string{
			"CUSTOM": "value",
		},
	}

	injector := NewInjector(
		WithDefaultVendor(VendorHuawei),
		WithSkipContainers("sidecar", "init"),
		WithCustomProfile(customProfile),
	)

	if injector.defaultVendor != VendorHuawei {
		t.Errorf("Default vendor = %q, want huawei", injector.defaultVendor)
	}

	if !injector.skipContainers["sidecar"] {
		t.Error("Skip container 'sidecar' not set")
	}

	if !injector.skipContainers["init"] {
		t.Error("Skip container 'init' not set")
	}

	if injector.customProfiles[VendorHuawei] == nil {
		t.Error("Custom profile not registered")
	}
}

func TestInjector_InjectPod_Nil(t *testing.T) {
	injector := NewInjector()
	_, err := injector.InjectPod(nil, VendorNVIDIA)
	if err == nil {
		t.Error("InjectPod(nil) should return error")
	}
}

func TestInjector_InjectPod_UnknownVendor(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	_, err := injector.InjectPod(pod, "unknown-vendor")
	if err == nil {
		t.Error("InjectPod with unknown vendor should return error")
	}
}

func TestInjector_InjectPod_NVIDIA(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "pytorch:latest",
				},
			},
		},
	}

	result, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	if !result.Injected {
		t.Error("Injection should have occurred")
	}

	if result.VendorUsed != VendorNVIDIA {
		t.Errorf("VendorUsed = %q, want nvidia", result.VendorUsed)
	}

	// Check env vars were injected
	container := &pod.Spec.Containers[0]
	foundVisibleDevices := false
	for _, env := range container.Env {
		if env.Name == "NVIDIA_VISIBLE_DEVICES" {
			foundVisibleDevices = true
		}
	}
	if !foundVisibleDevices {
		t.Error("NVIDIA_VISIBLE_DEVICES not injected")
	}

	// Check volumes were added
	if len(pod.Spec.Volumes) == 0 {
		t.Error("No volumes added to pod")
	}

	// Check volume mounts were added
	if len(container.VolumeMounts) == 0 {
		t.Error("No volume mounts added to container")
	}

	// Check runtime class was set
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "nvidia" {
		t.Error("RuntimeClassName not set correctly")
	}

	// Check annotations
	if pod.Annotations == nil || pod.Annotations["hcs.io/vendor"] != "nvidia" {
		t.Error("Annotations not set correctly")
	}
}

func TestInjector_InjectPod_Huawei(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	result, err := injector.InjectPod(pod, VendorHuawei)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	if result.VendorUsed != VendorHuawei {
		t.Errorf("VendorUsed = %q, want huawei", result.VendorUsed)
	}

	// Check Ascend-specific env var
	container := &pod.Spec.Containers[0]
	foundAscend := false
	for _, env := range container.Env {
		if env.Name == "ASCEND_VISIBLE_DEVICES" {
			foundAscend = true
		}
	}
	if !foundAscend {
		t.Error("ASCEND_VISIBLE_DEVICES not injected")
	}
}

func TestInjector_InjectPod_SkipContainer(t *testing.T) {
	injector := NewInjector(WithSkipContainers("sidecar"))
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
				{Name: "sidecar"},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	// Main container should have env vars
	mainContainer := &pod.Spec.Containers[0]
	if len(mainContainer.Env) == 0 {
		t.Error("Main container should have env vars")
	}

	// Sidecar should not have env vars
	sidecarContainer := &pod.Spec.Containers[1]
	if len(sidecarContainer.Env) > 0 {
		t.Error("Sidecar container should not have env vars")
	}
}

func TestInjector_InjectPod_InitContainers(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init"},
			},
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	// Init container should also have env vars
	initContainer := &pod.Spec.InitContainers[0]
	if len(initContainer.Env) == 0 {
		t.Error("Init container should have env vars")
	}
}

func TestInjector_InjectPod_ExistingEnvVars(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "main",
					Env: []corev1.EnvVar{
						{Name: "NVIDIA_VISIBLE_DEVICES", Value: "0"},
						{Name: "EXISTING_VAR", Value: "keep"},
					},
				},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	container := &pod.Spec.Containers[0]

	// Existing NVIDIA_VISIBLE_DEVICES should not be overwritten
	for _, env := range container.Env {
		if env.Name == "NVIDIA_VISIBLE_DEVICES" {
			if env.Value != "0" {
				t.Errorf("NVIDIA_VISIBLE_DEVICES was overwritten: %q", env.Value)
			}
		}
		if env.Name == "EXISTING_VAR" {
			if env.Value != "keep" {
				t.Error("EXISTING_VAR was modified")
			}
		}
	}
}

func TestInjector_InjectPod_ExistingPath(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "main",
					Env: []corev1.EnvVar{
						{Name: "PATH", Value: "/usr/bin"},
					},
				},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	container := &pod.Spec.Containers[0]
	for _, env := range container.Env {
		if env.Name == "PATH" {
			// PATH should be prepended, not replaced
			if env.Value == "/usr/bin" {
				t.Error("PATH was not modified")
			}
			if len(env.Value) < len("/usr/bin") {
				t.Error("PATH should be longer after prepending")
			}
		}
	}
}

func TestInjector_InjectPod_ExistingVolumes(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "main",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "existing", MountPath: "/existing"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "existing"},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	// Existing volume should still be present
	foundExisting := false
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "existing" {
			foundExisting = true
		}
	}
	if !foundExisting {
		t.Error("Existing volume was removed")
	}
}

func TestInjector_InjectPod_RuntimeClassPreserved(t *testing.T) {
	injector := NewInjector()
	existingRuntime := "custom-runtime"
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &existingRuntime,
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	_, err := injector.InjectPod(pod, VendorNVIDIA)
	if err != nil {
		t.Fatalf("InjectPod error: %v", err)
	}

	// Existing runtime class should be preserved
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "custom-runtime" {
		t.Error("Existing RuntimeClassName was overwritten")
	}
}

func TestInjector_InjectPodByNodeLabels(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	nodeLabels := map[string]string{
		"hcs.io/vendor": "huawei",
	}

	result, err := injector.InjectPodByNodeLabels(pod, nodeLabels)
	if err != nil {
		t.Fatalf("InjectPodByNodeLabels error: %v", err)
	}

	if result.VendorUsed != VendorHuawei {
		t.Errorf("VendorUsed = %q, want huawei", result.VendorUsed)
	}
}

func TestInjector_InjectPodByAnnotations(t *testing.T) {
	injector := NewInjector()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"hcs.io/vendor": "hygon",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "main"},
			},
		},
	}

	result, err := injector.InjectPodByAnnotations(pod)
	if err != nil {
		t.Fatalf("InjectPodByAnnotations error: %v", err)
	}

	if result.VendorUsed != VendorHygon {
		t.Errorf("VendorUsed = %q, want hygon", result.VendorUsed)
	}
}

func TestInjector_detectVendorFromLabels(t *testing.T) {
	injector := NewInjector()

	tests := []struct {
		name     string
		labels   map[string]string
		expected VendorType
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: VendorNVIDIA, // default
		},
		{
			name: "hcs.io/vendor label",
			labels: map[string]string{
				"hcs.io/vendor": "huawei",
			},
			expected: VendorHuawei,
		},
		{
			name: "nvidia presence label",
			labels: map[string]string{
				"nvidia.com/gpu.present": "true",
			},
			expected: VendorNVIDIA,
		},
		{
			name: "huawei presence label",
			labels: map[string]string{
				"huawei.com/npu": "true",
			},
			expected: VendorHuawei,
		},
		{
			name: "hygon presence label",
			labels: map[string]string{
				"hygon.com/dcu": "true",
			},
			expected: VendorHygon,
		},
		{
			name: "cambricon presence label",
			labels: map[string]string{
				"cambricon.com/mlu": "true",
			},
			expected: VendorCambricon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injector.detectVendorFromLabels(tt.labels)
			if result != tt.expected {
				t.Errorf("detectVendorFromLabels() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestNeedsInjection(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: false,
		},
		{
			name: "no resources",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main"},
					},
				},
			},
			expected: false,
		},
		{
			name: "ai.compute resource request",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"ai.compute/vram": resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "nvidia.com/gpu resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "huawei.com/npu resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"huawei.com/npu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "inject annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"hcs.io/inject": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "main"},
					},
				},
			},
			expected: true,
		},
		{
			name: "init container with ai.compute",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "init",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"ai.compute/tflops": resource.MustParse("100"),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{Name: "main"},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsInjection(tt.pod)
			if result != tt.expected {
				t.Errorf("NeedsInjection() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectVendorFromResources(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected VendorType
	}{
		{
			name:     "nil pod",
			pod:      nil,
			expected: "",
		},
		{
			name: "no vendor resources",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "nvidia resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: VendorNVIDIA,
		},
		{
			name: "huawei resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"huawei.com/npu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: VendorHuawei,
		},
		{
			name: "hygon resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"hygon.com/dcu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: VendorHygon,
		},
		{
			name: "cambricon resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"cambricon.com/mlu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
			expected: VendorCambricon,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectVendorFromResources(tt.pod)
			if result != tt.expected {
				t.Errorf("DetectVendorFromResources() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestDefaultWebhookConfig(t *testing.T) {
	config := DefaultWebhookConfig()

	if config.Port != 9443 {
		t.Errorf("Port = %d, want 9443", config.Port)
	}

	if config.MutatePath != "/mutate-v1-pod" {
		t.Errorf("MutatePath = %q, want /mutate-v1-pod", config.MutatePath)
	}

	if config.ValidatePath != "/validate-v1-pod" {
		t.Errorf("ValidatePath = %q, want /validate-v1-pod", config.ValidatePath)
	}
}

func TestNewPodMutator(t *testing.T) {
	mutator := NewPodMutator(nil)
	if mutator == nil {
		t.Fatal("NewPodMutator() returned nil")
	}

	if mutator.Injector == nil {
		t.Error("Injector should be initialized")
	}

	if mutator.HCSInjector == nil {
		t.Error("HCSInjector should be initialized")
	}
}

func TestNewPodMutator_WithInjector(t *testing.T) {
	injector := NewInjector(WithDefaultVendor(VendorHuawei))
	mutator := NewPodMutator(nil, WithPodMutatorInjector(injector))

	if mutator.Injector != injector {
		t.Error("Mutator should use provided injector")
	}
}

func TestNewPodMutator_WithHCSInjector(t *testing.T) {
	hcsInjector := NewHCSInjector(WithInterceptorPath("/custom/path/interceptor.so"))
	mutator := NewPodMutator(nil, WithPodMutatorHCSInjector(hcsInjector))

	if mutator.HCSInjector != hcsInjector {
		t.Error("Mutator should use provided HCS injector")
	}
}

func TestNewPodValidator(t *testing.T) {
	validator := NewPodValidator(nil)
	if validator == nil {
		t.Fatal("NewPodValidator() returned nil")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestInjector_Concurrency(t *testing.T) {
	injector := NewInjector()
	done := make(chan bool)

	// Concurrent injections
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 50; j++ {
				pod := &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "main"},
						},
					},
				}
				vendors := []VendorType{VendorNVIDIA, VendorHuawei, VendorHygon, VendorCambricon}
				vendor := vendors[j%len(vendors)]
				injector.InjectPod(pod, vendor)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
