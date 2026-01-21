package detectors

import (
	"context"
)

// Detector 硬件检测器接口
type Detector interface {
	// Name 返回检测器名称
	Name() string

	// Detect 检测硬件类型和可用性
	Detect(ctx context.Context) (*HardwareType, error)

	// GetDevices 获取设备列表
	GetDevices(ctx context.Context) ([]*Device, error)
}

// HardwareType 硬件类型信息
type HardwareType struct {
	Vendor          string // nvidia, huawei, hygon, cambricon
	DriverAvailable bool   // 驱动是否可用
}

// Device 设备信息
type Device struct {
	ID         string
	Model      string
	VRAMTotal  uint64 // bytes
	VRAMFree   uint64 // bytes
	PCIEBusID  string
	ComputeCap ComputeCapability
}

// ComputeCapability 计算能力
type ComputeCapability struct {
	TFP16FLOPS uint64 // TFLOPS
	TFP32FLOPS uint64 // TFLOPS
}
