package agent

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/collectors"
	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// Reporter CRD 上报器
type Reporter struct {
	client client.Client
}

// NewReporter 创建上报器
func NewReporter(c client.Client) *Reporter {
	return &Reporter{
		client: c,
	}
}

// Report 上报节点状态
func (r *Reporter) Report(ctx context.Context, nodeName string, hwType *detectors.HardwareType, devices []*detectors.Device, metrics *collectors.Metrics) error {
	// 构建 ComputeNode 对象
	cn := r.buildComputeNode(nodeName, hwType, devices, metrics)

	// 尝试获取现有的 ComputeNode
	existing := &v1alpha1.ComputeNode{}
	err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			// 创建新的 ComputeNode
			klog.Infof("Creating ComputeNode: %s", nodeName)
			if err := r.client.Create(ctx, cn); err != nil {
				return fmt.Errorf("failed to create ComputeNode: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ComputeNode: %w", err)
	}

	// 更新现有的 ComputeNode
	existing.Spec = cn.Spec
	existing.Status = cn.Status

	klog.V(2).Infof("Updating ComputeNode: %s", nodeName)
	if err := r.client.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update ComputeNode: %w", err)
	}

	// 更新状态（需要单独调用 Status().Update()）
	if err := r.client.Status().Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update ComputeNode status: %w", err)
	}

	return nil
}

// buildComputeNode 构建 ComputeNode 对象
func (r *Reporter) buildComputeNode(nodeName string, hwType *detectors.HardwareType, devices []*detectors.Device, metrics *collectors.Metrics) *v1alpha1.ComputeNode {
	cn := &v1alpha1.ComputeNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: v1alpha1.ComputeNodeSpec{
			NodeName: nodeName,
			Vendor:   hwType.Vendor,
		},
		Status: v1alpha1.ComputeNodeStatus{
			Phase:      v1alpha1.ComputeNodePhaseReady,
			Devices:    make([]v1alpha1.DeviceInfo, 0, len(devices)),
			Conditions: make([]v1alpha1.ComputeNodeCondition, 0),
		},
	}

	// 填充 TotalCapacity
	if metrics.Fingerprint != nil {
		cn.Spec.TotalCapacity = v1alpha1.ComputeCapacity{
			VRAM:       metrics.Fingerprint.VRAMTotal,
			FP16TFLOPS: metrics.Fingerprint.ComputeCap.FP16TFLOPS,
			FP32TFLOPS: metrics.Fingerprint.ComputeCap.FP32TFLOPS,
		}
	}

	// 填充设备信息
	for _, dev := range devices {
		deviceInfo := v1alpha1.DeviceInfo{
			ID:               dev.ID,
			Model:            dev.Model,
			VRAMTotal:        dev.VRAMTotal,
			VRAMUsed:         dev.VRAMUsed,
			HealthScore:      dev.HealthScore,
			PCIEBusID:        dev.PCIEBusID,
			InterconnectType: string(detectors.LinkTypePCIe), // 默认 PCIe
		}
		cn.Status.Devices = append(cn.Status.Devices, deviceInfo)
	}

	// 设置条件
	now := metav1.NewTime(time.Now())

	// DriverAvailable 条件
	driverCondition := v1alpha1.ComputeNodeCondition{
		Type:               v1alpha1.ComputeNodeConditionDriverAvailable,
		LastTransitionTime: now,
	}
	if hwType.DriverAvailable {
		driverCondition.Status = corev1.ConditionTrue
		driverCondition.Reason = "DriverReady"
		driverCondition.Message = fmt.Sprintf("Driver version: %s", hwType.DriverVersion)
	} else {
		driverCondition.Status = corev1.ConditionFalse
		driverCondition.Reason = "DriverNotAvailable"
		driverCondition.Message = "Hardware driver is not available"
	}
	cn.Status.Conditions = append(cn.Status.Conditions, driverCondition)

	// DevicesReady 条件
	devicesCondition := v1alpha1.ComputeNodeCondition{
		Type:               v1alpha1.ComputeNodeConditionDevicesReady,
		LastTransitionTime: now,
	}
	if len(devices) > 0 {
		devicesCondition.Status = corev1.ConditionTrue
		devicesCondition.Reason = "DevicesDetected"
		devicesCondition.Message = fmt.Sprintf("%d device(s) detected", len(devices))
	} else {
		devicesCondition.Status = corev1.ConditionFalse
		devicesCondition.Reason = "NoDevices"
		devicesCondition.Message = "No compute devices detected"
	}
	cn.Status.Conditions = append(cn.Status.Conditions, devicesCondition)

	// Healthy 条件
	healthCondition := v1alpha1.ComputeNodeCondition{
		Type:               v1alpha1.ComputeNodeConditionHealthy,
		LastTransitionTime: now,
	}
	if metrics.Health != nil && metrics.Health.Score >= 60 {
		healthCondition.Status = corev1.ConditionTrue
		healthCondition.Reason = "Healthy"
		healthCondition.Message = fmt.Sprintf("Health score: %.1f", metrics.Health.Score)
		cn.Status.Phase = v1alpha1.ComputeNodePhaseReady
	} else if metrics.Health != nil && metrics.Health.Score >= 30 {
		healthCondition.Status = corev1.ConditionTrue
		healthCondition.Reason = "Warning"
		healthCondition.Message = fmt.Sprintf("Health score: %.1f (degraded)", metrics.Health.Score)
		cn.Status.Phase = v1alpha1.ComputeNodePhaseReady
	} else {
		healthCondition.Status = corev1.ConditionFalse
		healthCondition.Reason = "Unhealthy"
		if metrics.Health != nil {
			healthCondition.Message = fmt.Sprintf("Health score: %.1f (critical)", metrics.Health.Score)
		} else {
			healthCondition.Message = "Health data unavailable"
		}
		cn.Status.Phase = v1alpha1.ComputeNodePhaseUnhealthy
	}
	cn.Status.Conditions = append(cn.Status.Conditions, healthCondition)

	return cn
}

// Delete 删除 ComputeNode
func (r *Reporter) Delete(ctx context.Context, nodeName string) error {
	cn := &v1alpha1.ComputeNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}

	if err := r.client.Delete(ctx, cn); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to delete ComputeNode: %w", err)
	}

	return nil
}
