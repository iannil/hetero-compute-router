package nvidia

import (
	"context"
	"fmt"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// NVMLDetector NVIDIA 硬件检测器
type NVMLDetector struct{}

// NewNVMLDetector 创建 NVIDIA 检测器
func NewNVMLDetector() detectors.Detector {
	return &NVMLDetector{}
}

// Name 返回检测器名称
func (d *NVMLDetector) Name() string {
	return "nvidia-nvml"
}

// Detect 检测 NVIDIA 硬件类型和可用性
func (d *NVMLDetector) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	// TODO: Phase 1 实现 - 调用 go-nvml 检测 NVIDIA GPU
	return &detectors.HardwareType{
		Vendor:          "nvidia",
		DriverAvailable: false,
	}, fmt.Errorf("not implemented yet")
}

// GetDevices 获取 NVIDIA 设备列表
func (d *NVMLDetector) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	// TODO: Phase 1 实现 - 获取 GPU 设备详情
	return nil, fmt.Errorf("not implemented yet")
}
