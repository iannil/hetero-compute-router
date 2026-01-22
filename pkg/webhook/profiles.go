package webhook

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// VendorType represents supported hardware vendors
type VendorType string

const (
	VendorNVIDIA    VendorType = "nvidia"
	VendorHuawei    VendorType = "huawei"
	VendorHygon     VendorType = "hygon"
	VendorCambricon VendorType = "cambricon"
)

// VolumeConfig defines a volume mount configuration for vendor drivers
type VolumeConfig struct {
	// Name is the volume name
	Name string
	// HostPath is the path on the host node
	HostPath string
	// MountPath is the path inside the container
	MountPath string
	// ReadOnly specifies if the mount should be read-only
	ReadOnly bool
}

// DeviceConfig defines device file configurations
type DeviceConfig struct {
	// HostPath is the device path on the host
	HostPath string
	// ContainerPath is the device path inside the container
	ContainerPath string
}

// VendorProfile defines vendor-specific configurations for environment injection
type VendorProfile struct {
	// Vendor is the hardware vendor identifier
	Vendor VendorType

	// EnvVars contains environment variables to inject
	EnvVars map[string]string

	// Volumes contains volume configurations to mount
	Volumes []VolumeConfig

	// Devices contains device file configurations
	Devices []DeviceConfig

	// RuntimeClassName is the optional runtime class for this vendor
	RuntimeClassName string

	// Annotations contains additional annotations to add to the pod
	Annotations map[string]string
}

// =============================================================================
// Pre-defined Vendor Profiles
// =============================================================================

// NVIDIAProfile defines the default configuration for NVIDIA GPUs
var NVIDIAProfile = VendorProfile{
	Vendor: VendorNVIDIA,
	EnvVars: map[string]string{
		"NVIDIA_VISIBLE_DEVICES":     "all",
		"NVIDIA_DRIVER_CAPABILITIES": "compute,utility",
		"LD_LIBRARY_PATH":            "/usr/local/nvidia/lib64:/usr/local/cuda/lib64",
		"PATH":                       "/usr/local/nvidia/bin:/usr/local/cuda/bin:$PATH",
	},
	Volumes: []VolumeConfig{
		{
			Name:      "nvidia-driver",
			HostPath:  "/usr/local/nvidia",
			MountPath: "/usr/local/nvidia",
			ReadOnly:  true,
		},
		{
			Name:      "nvidia-cuda",
			HostPath:  "/usr/local/cuda",
			MountPath: "/usr/local/cuda",
			ReadOnly:  true,
		},
	},
	Devices: []DeviceConfig{
		{
			HostPath:      "/dev/nvidia0",
			ContainerPath: "/dev/nvidia0",
		},
		{
			HostPath:      "/dev/nvidiactl",
			ContainerPath: "/dev/nvidiactl",
		},
		{
			HostPath:      "/dev/nvidia-uvm",
			ContainerPath: "/dev/nvidia-uvm",
		},
	},
	RuntimeClassName: "nvidia",
	Annotations: map[string]string{
		"hcs.io/vendor": "nvidia",
	},
}

// AscendProfile defines the default configuration for Huawei Ascend NPUs
var AscendProfile = VendorProfile{
	Vendor: VendorHuawei,
	EnvVars: map[string]string{
		"ASCEND_VISIBLE_DEVICES":      "all",
		"LD_LIBRARY_PATH":             "/usr/local/Ascend/driver/lib64:/usr/local/Ascend/ascend-toolkit/latest/lib64",
		"PATH":                        "/usr/local/Ascend/ascend-toolkit/latest/bin:$PATH",
		"ASCEND_AICPU_PATH":           "/usr/local/Ascend/ascend-toolkit/latest",
		"ASCEND_OPP_PATH":             "/usr/local/Ascend/ascend-toolkit/latest/opp",
		"TOOLCHAIN_HOME":              "/usr/local/Ascend/ascend-toolkit/latest/toolkit",
		"ASCEND_HOME_PATH":            "/usr/local/Ascend/ascend-toolkit/latest",
		"LD_PRELOAD":                  "/usr/local/Ascend/driver/lib64/libgomp.so.1",
		"ASCEND_GLOBAL_LOG_LEVEL":     "3",
		"ASCEND_SLOG_PRINT_TO_STDOUT": "0",
	},
	Volumes: []VolumeConfig{
		{
			Name:      "ascend-driver",
			HostPath:  "/usr/local/Ascend/driver",
			MountPath: "/usr/local/Ascend/driver",
			ReadOnly:  true,
		},
		{
			Name:      "ascend-toolkit",
			HostPath:  "/usr/local/Ascend/ascend-toolkit",
			MountPath: "/usr/local/Ascend/ascend-toolkit",
			ReadOnly:  true,
		},
	},
	Devices: []DeviceConfig{
		{
			HostPath:      "/dev/davinci0",
			ContainerPath: "/dev/davinci0",
		},
		{
			HostPath:      "/dev/davinci_manager",
			ContainerPath: "/dev/davinci_manager",
		},
		{
			HostPath:      "/dev/devmm_svm",
			ContainerPath: "/dev/devmm_svm",
		},
		{
			HostPath:      "/dev/hisi_hdc",
			ContainerPath: "/dev/hisi_hdc",
		},
	},
	RuntimeClassName: "",
	Annotations: map[string]string{
		"hcs.io/vendor": "huawei",
	},
}

// HygonProfile defines the default configuration for Hygon DCU
var HygonProfile = VendorProfile{
	Vendor: VendorHygon,
	EnvVars: map[string]string{
		"DCU_VISIBLE_DEVICES": "all",
		"LD_LIBRARY_PATH":     "/opt/hygon/lib:/opt/dtk/lib",
		"PATH":                "/opt/dtk/bin:$PATH",
		"HIP_PLATFORM":        "hcc",
		"HSA_ENABLE_SDMA":     "0",
	},
	Volumes: []VolumeConfig{
		{
			Name:      "hygon-driver",
			HostPath:  "/opt/hygon",
			MountPath: "/opt/hygon",
			ReadOnly:  true,
		},
		{
			Name:      "hygon-dtk",
			HostPath:  "/opt/dtk",
			MountPath: "/opt/dtk",
			ReadOnly:  true,
		},
	},
	Devices: []DeviceConfig{
		{
			HostPath:      "/dev/kfd",
			ContainerPath: "/dev/kfd",
		},
		{
			HostPath:      "/dev/dri",
			ContainerPath: "/dev/dri",
		},
	},
	RuntimeClassName: "",
	Annotations: map[string]string{
		"hcs.io/vendor": "hygon",
	},
}

// CambriconProfile defines the default configuration for Cambricon MLU
var CambriconProfile = VendorProfile{
	Vendor: VendorCambricon,
	EnvVars: map[string]string{
		"MLU_VISIBLE_DEVICES": "all",
		"LD_LIBRARY_PATH":     "/usr/local/neuware/lib64",
		"PATH":                "/usr/local/neuware/bin:$PATH",
		"NEUWARE_HOME":        "/usr/local/neuware",
	},
	Volumes: []VolumeConfig{
		{
			Name:      "cambricon-driver",
			HostPath:  "/usr/local/neuware",
			MountPath: "/usr/local/neuware",
			ReadOnly:  true,
		},
	},
	Devices: []DeviceConfig{
		{
			HostPath:      "/dev/cambricon_dev0",
			ContainerPath: "/dev/cambricon_dev0",
		},
		{
			HostPath:      "/dev/cambricon_ctl",
			ContainerPath: "/dev/cambricon_ctl",
		},
	},
	RuntimeClassName: "",
	Annotations: map[string]string{
		"hcs.io/vendor": "cambricon",
	},
}

// =============================================================================
// Profile Registry
// =============================================================================

// profileRegistry maps vendor types to their profiles
var profileRegistry = map[VendorType]*VendorProfile{
	VendorNVIDIA:    &NVIDIAProfile,
	VendorHuawei:    &AscendProfile,
	VendorHygon:     &HygonProfile,
	VendorCambricon: &CambriconProfile,
}

// GetProfile returns the vendor profile for a given vendor type
func GetProfile(vendor VendorType) *VendorProfile {
	if profile, ok := profileRegistry[vendor]; ok {
		return profile
	}
	return nil
}

// GetProfileByString returns the vendor profile by vendor string
func GetProfileByString(vendor string) *VendorProfile {
	return GetProfile(VendorType(vendor))
}

// RegisterProfile registers or updates a vendor profile in the registry
func RegisterProfile(profile *VendorProfile) {
	if profile != nil {
		profileRegistry[profile.Vendor] = profile
	}
}

// ListProfiles returns all registered vendor profiles
func ListProfiles() []*VendorProfile {
	profiles := make([]*VendorProfile, 0, len(profileRegistry))
	for _, p := range profileRegistry {
		profiles = append(profiles, p)
	}
	return profiles
}

// ListVendors returns all registered vendor types
func ListVendors() []VendorType {
	vendors := make([]VendorType, 0, len(profileRegistry))
	for v := range profileRegistry {
		vendors = append(vendors, v)
	}
	return vendors
}

// =============================================================================
// Kubernetes Resource Builders
// =============================================================================

// BuildEnvVars converts the profile's environment variables to Kubernetes EnvVar slice
func (p *VendorProfile) BuildEnvVars() []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0, len(p.EnvVars))
	for name, value := range p.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
	return envVars
}

// BuildVolumes converts the profile's volume configs to Kubernetes Volume slice
func (p *VendorProfile) BuildVolumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(p.Volumes))
	for _, vc := range p.Volumes {
		hostPathType := corev1.HostPathDirectory
		volumes = append(volumes, corev1.Volume{
			Name: vc.Name,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: vc.HostPath,
					Type: &hostPathType,
				},
			},
		})
	}
	return volumes
}

// BuildVolumeMounts converts the profile's volume configs to Kubernetes VolumeMount slice
func (p *VendorProfile) BuildVolumeMounts() []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(p.Volumes))
	for _, vc := range p.Volumes {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      vc.Name,
			MountPath: vc.MountPath,
			ReadOnly:  vc.ReadOnly,
		})
	}
	return mounts
}

// BuildDeviceVolumes builds volumes for device files
func (p *VendorProfile) BuildDeviceVolumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(p.Devices))
	for i, dc := range p.Devices {
		hostPathType := corev1.HostPathCharDev
		volumeName := fmt.Sprintf("%s-device-%d", p.Vendor, i)
		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: dc.HostPath,
					Type: &hostPathType,
				},
			},
		})
	}
	return volumes
}

// BuildDeviceVolumeMounts builds volume mounts for device files
func (p *VendorProfile) BuildDeviceVolumeMounts() []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, len(p.Devices))
	for i, dc := range p.Devices {
		volumeName := fmt.Sprintf("%s-device-%d", p.Vendor, i)
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: dc.ContainerPath,
		})
	}
	return mounts
}

// HasRuntimeClass returns true if the profile specifies a runtime class
func (p *VendorProfile) HasRuntimeClass() bool {
	return p.RuntimeClassName != ""
}

// Clone creates a deep copy of the VendorProfile
func (p *VendorProfile) Clone() *VendorProfile {
	if p == nil {
		return nil
	}

	clone := &VendorProfile{
		Vendor:           p.Vendor,
		RuntimeClassName: p.RuntimeClassName,
	}

	// Clone EnvVars
	if p.EnvVars != nil {
		clone.EnvVars = make(map[string]string, len(p.EnvVars))
		for k, v := range p.EnvVars {
			clone.EnvVars[k] = v
		}
	}

	// Clone Volumes
	if p.Volumes != nil {
		clone.Volumes = make([]VolumeConfig, len(p.Volumes))
		copy(clone.Volumes, p.Volumes)
	}

	// Clone Devices
	if p.Devices != nil {
		clone.Devices = make([]DeviceConfig, len(p.Devices))
		copy(clone.Devices, p.Devices)
	}

	// Clone Annotations
	if p.Annotations != nil {
		clone.Annotations = make(map[string]string, len(p.Annotations))
		for k, v := range p.Annotations {
			clone.Annotations[k] = v
		}
	}

	return clone
}
