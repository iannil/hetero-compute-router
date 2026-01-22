package webhook

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewHCSInjector(t *testing.T) {
	tests := []struct {
		name     string
		opts     []HCSInjectorOption
		wantPath string
		wantHost string
	}{
		{
			name:     "default values",
			opts:     nil,
			wantPath: "/usr/local/hcs/lib/libhcs_interceptor.so",
			wantHost: "/usr/local/hcs/lib",
		},
		{
			name: "custom interceptor path",
			opts: []HCSInjectorOption{
				WithInterceptorPath("/custom/path/interceptor.so"),
			},
			wantPath: "/custom/path/interceptor.so",
			wantHost: "/usr/local/hcs/lib",
		},
		{
			name: "custom host lib path",
			opts: []HCSInjectorOption{
				WithHostLibPath("/custom/host/lib"),
			},
			wantPath: "/usr/local/hcs/lib/libhcs_interceptor.so",
			wantHost: "/custom/host/lib",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewHCSInjector(tt.opts...)
			if injector.GetInterceptorPath() != tt.wantPath {
				t.Errorf("interceptorPath = %v, want %v", injector.GetInterceptorPath(), tt.wantPath)
			}
			if injector.GetHostLibPath() != tt.wantHost {
				t.Errorf("hostLibPath = %v, want %v", injector.GetHostLibPath(), tt.wantHost)
			}
		})
	}
}

func TestNeedsHCSInjection(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "nil pod",
			pod:  nil,
			want: false,
		},
		{
			name: "pod with ai.compute/vram request",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod with ai.compute/vram limit",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("8Gi"),
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "pod without ai.compute/vram",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "pod with explicit annotation true",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HCSInjectAnnotation: "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test"},
					},
				},
			},
			want: true,
		},
		{
			name: "init container with ai.compute/vram",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "init",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("4Gi"),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{Name: "main"},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsHCSInjection(tt.pod); got != tt.want {
				t.Errorf("NeedsHCSInjection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHCSInjector_InjectHCS(t *testing.T) {
	tests := []struct {
		name              string
		pod               *corev1.Pod
		opts              []HCSInjectorOption
		wantInjected      bool
		wantVRAMQuota     string
		wantContainersCnt int
		wantErr           bool
	}{
		{
			name:    "nil pod",
			pod:     nil,
			wantErr: true,
		},
		{
			name: "pod without vram - no injection",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test"},
					},
				},
			},
			wantInjected: false,
		},
		{
			name: "pod with vram request - injection",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			wantInjected:      true,
			wantVRAMQuota:     "16Gi",
			wantContainersCnt: 1,
		},
		{
			name: "multi-container pod",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("8Gi"),
								},
							},
						},
						{
							Name: "sidecar",
						},
					},
				},
			},
			wantInjected:      true,
			wantVRAMQuota:     "8Gi",
			wantContainersCnt: 2,
		},
		{
			name: "explicit disable annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						HCSInjectAnnotation: "false",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			wantInjected: false,
		},
		{
			name: "skip specified container",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "main",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("8Gi"),
								},
							},
						},
						{
							Name: "skip-me",
						},
					},
				},
			},
			opts: []HCSInjectorOption{
				WithHCSSkipContainers("skip-me"),
			},
			wantInjected:      true,
			wantVRAMQuota:     "8Gi",
			wantContainersCnt: 1,
		},
		{
			name: "disabled injector",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			opts: []HCSInjectorOption{
				WithHCSEnabled(false),
			},
			wantInjected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewHCSInjector(tt.opts...)
			result, err := injector.InjectHCS(tt.pod)

			if (err != nil) != tt.wantErr {
				t.Errorf("InjectHCS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if result.Injected != tt.wantInjected {
				t.Errorf("Injected = %v, want %v", result.Injected, tt.wantInjected)
			}

			if result.VRAMQuota != tt.wantVRAMQuota {
				t.Errorf("VRAMQuota = %v, want %v", result.VRAMQuota, tt.wantVRAMQuota)
			}

			if result.ContainersInjected != tt.wantContainersCnt {
				t.Errorf("ContainersInjected = %v, want %v", result.ContainersInjected, tt.wantContainersCnt)
			}
		})
	}
}

func TestHCSInjector_InjectLDPreload(t *testing.T) {
	injector := NewHCSInjector()

	tests := []struct {
		name       string
		existingLD string
		wantLD     string
	}{
		{
			name:       "no existing LD_PRELOAD",
			existingLD: "",
			wantLD:     "/usr/local/hcs/lib/libhcs_interceptor.so",
		},
		{
			name:       "with existing LD_PRELOAD",
			existingLD: "/other/lib.so",
			wantLD:     "/usr/local/hcs/lib/libhcs_interceptor.so:/other/lib.so",
		},
		{
			name:       "already has interceptor",
			existingLD: "/usr/local/hcs/lib/libhcs_interceptor.so:/other/lib.so",
			wantLD:     "/usr/local/hcs/lib/libhcs_interceptor.so:/other/lib.so",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{Name: "test"}
			if tt.existingLD != "" {
				container.Env = []corev1.EnvVar{
					{Name: LDPreloadEnvVar, Value: tt.existingLD},
				}
			}

			injector.injectLDPreload(container)

			// Find LD_PRELOAD
			var ldPreload string
			for _, env := range container.Env {
				if env.Name == LDPreloadEnvVar {
					ldPreload = env.Value
					break
				}
			}

			if ldPreload != tt.wantLD {
				t.Errorf("LD_PRELOAD = %v, want %v", ldPreload, tt.wantLD)
			}
		})
	}
}

func TestHCSInjector_InjectVRAMQuota(t *testing.T) {
	injector := NewHCSInjector()

	tests := []struct {
		name         string
		existingEnvs []corev1.EnvVar
		quota        string
		wantQuota    string
	}{
		{
			name:      "no existing quota",
			quota:     "16Gi",
			wantQuota: "16Gi",
		},
		{
			name: "update existing quota",
			existingEnvs: []corev1.EnvVar{
				{Name: HCSVRAMQuotaEnvVar, Value: "8Gi"},
			},
			quota:     "16Gi",
			wantQuota: "16Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				Name: "test",
				Env:  tt.existingEnvs,
			}

			injector.injectVRAMQuota(container, tt.quota)

			// Find HCS_VRAM_QUOTA
			var quotaValue string
			for _, env := range container.Env {
				if env.Name == HCSVRAMQuotaEnvVar {
					quotaValue = env.Value
					break
				}
			}

			if quotaValue != tt.wantQuota {
				t.Errorf("HCS_VRAM_QUOTA = %v, want %v", quotaValue, tt.wantQuota)
			}
		})
	}
}

func TestHCSInjector_InjectVolume(t *testing.T) {
	injector := NewHCSInjector()

	t.Run("add volume to pod", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "test"}},
			},
		}

		injector.injectVolume(pod)

		if len(pod.Spec.Volumes) != 1 {
			t.Errorf("expected 1 volume, got %d", len(pod.Spec.Volumes))
			return
		}

		vol := pod.Spec.Volumes[0]
		if vol.Name != HCSLibVolumeName {
			t.Errorf("volume name = %v, want %v", vol.Name, HCSLibVolumeName)
		}
		if vol.HostPath == nil {
			t.Error("expected HostPath volume source")
			return
		}
		if vol.HostPath.Path != "/usr/local/hcs/lib" {
			t.Errorf("hostPath = %v, want /usr/local/hcs/lib", vol.HostPath.Path)
		}
	})

	t.Run("volume already exists", func(t *testing.T) {
		hostPathType := corev1.HostPathDirectory
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "test"}},
				Volumes: []corev1.Volume{
					{
						Name: HCSLibVolumeName,
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/existing/path",
								Type: &hostPathType,
							},
						},
					},
				},
			},
		}

		injector.injectVolume(pod)

		if len(pod.Spec.Volumes) != 1 {
			t.Errorf("expected 1 volume, got %d", len(pod.Spec.Volumes))
		}
		// Should not modify existing volume
		if pod.Spec.Volumes[0].HostPath.Path != "/existing/path" {
			t.Errorf("existing volume was modified")
		}
	})
}

func TestHCSInjector_InjectVolumeMount(t *testing.T) {
	injector := NewHCSInjector()

	t.Run("add volume mount to container", func(t *testing.T) {
		container := &corev1.Container{Name: "test"}

		injector.injectVolumeMount(container)

		if len(container.VolumeMounts) != 1 {
			t.Errorf("expected 1 volume mount, got %d", len(container.VolumeMounts))
			return
		}

		mount := container.VolumeMounts[0]
		if mount.Name != HCSLibVolumeName {
			t.Errorf("mount name = %v, want %v", mount.Name, HCSLibVolumeName)
		}
		if mount.MountPath != "/usr/local/hcs/lib" {
			t.Errorf("mountPath = %v, want /usr/local/hcs/lib", mount.MountPath)
		}
		if !mount.ReadOnly {
			t.Error("expected read-only mount")
		}
	})

	t.Run("mount already exists", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test",
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      HCSLibVolumeName,
					MountPath: "/existing/path",
				},
			},
		}

		injector.injectVolumeMount(container)

		if len(container.VolumeMounts) != 1 {
			t.Errorf("expected 1 volume mount, got %d", len(container.VolumeMounts))
		}
		// Should not modify existing mount
		if container.VolumeMounts[0].MountPath != "/existing/path" {
			t.Errorf("existing mount was modified")
		}
	})
}

func TestHCSInjector_ExtractVRAMQuota(t *testing.T) {
	injector := NewHCSInjector()

	tests := []struct {
		name      string
		pod       *corev1.Pod
		wantQuota string
	}{
		{
			name: "quota from annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"hcs.io/vram-quota": "32Gi",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			wantQuota: "32Gi",
		},
		{
			name: "quota from request",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
								},
							},
						},
					},
				},
			},
			wantQuota: "16Gi",
		},
		{
			name: "max quota from multiple containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "small",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("8Gi"),
								},
							},
						},
						{
							Name: "large",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceName(HCSVRAMResource): resource.MustParse("32Gi"),
								},
							},
						},
					},
				},
			},
			wantQuota: "32Gi",
		},
		{
			name: "no vram resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test"},
					},
				},
			},
			wantQuota: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quota := injector.extractVRAMQuota(tt.pod)
			if quota != tt.wantQuota {
				t.Errorf("extractVRAMQuota() = %v, want %v", quota, tt.wantQuota)
			}
		})
	}
}

func TestHCSInjector_FullInjection(t *testing.T) {
	// Test full injection flow
	injector := NewHCSInjector()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init"},
			},
			Containers: []corev1.Container{
				{
					Name: "main",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(HCSVRAMResource): resource.MustParse("16Gi"),
						},
					},
				},
			},
		},
	}

	result, err := injector.InjectHCS(pod)
	if err != nil {
		t.Fatalf("InjectHCS() error = %v", err)
	}

	if !result.Injected {
		t.Error("expected injection to occur")
	}

	// Check main container
	mainContainer := pod.Spec.Containers[0]

	// Check LD_PRELOAD
	var ldPreload string
	for _, env := range mainContainer.Env {
		if env.Name == LDPreloadEnvVar {
			ldPreload = env.Value
			break
		}
	}
	if ldPreload != "/usr/local/hcs/lib/libhcs_interceptor.so" {
		t.Errorf("LD_PRELOAD = %v, want interceptor path", ldPreload)
	}

	// Check HCS_VRAM_QUOTA
	var vramQuota string
	for _, env := range mainContainer.Env {
		if env.Name == HCSVRAMQuotaEnvVar {
			vramQuota = env.Value
			break
		}
	}
	if vramQuota != "16Gi" {
		t.Errorf("HCS_VRAM_QUOTA = %v, want 16Gi", vramQuota)
	}

	// Check volume mount
	var hasMount bool
	for _, mount := range mainContainer.VolumeMounts {
		if mount.Name == HCSLibVolumeName {
			hasMount = true
			break
		}
	}
	if !hasMount {
		t.Error("expected volume mount")
	}

	// Check volume
	var hasVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == HCSLibVolumeName {
			hasVolume = true
			break
		}
	}
	if !hasVolume {
		t.Error("expected volume")
	}

	// Check init container was also injected
	initContainer := pod.Spec.InitContainers[0]
	var initHasLD bool
	for _, env := range initContainer.Env {
		if env.Name == LDPreloadEnvVar {
			initHasLD = true
			break
		}
	}
	if !initHasLD {
		t.Error("expected init container to have LD_PRELOAD")
	}

	// Check annotation
	if pod.Annotations[HCSInjectAnnotation] != "true" {
		t.Error("expected injection annotation")
	}
}
