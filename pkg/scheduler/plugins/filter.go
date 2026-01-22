package plugins

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/framework"
)

const (
	// PluginName 插件名称
	PluginName = "HCSComputeFilter"

	// VRAMResourceName VRAM 资源名称
	VRAMResourceName = "ai.compute/vram"

	// FP16TFLOPSResourceName FP16 TFLOPS 资源名称
	FP16TFLOPSResourceName = "ai.compute/tflops-fp16"
)

// FilterPlugin HCS 过滤插件
type FilterPlugin struct {
	client client.Client
}

var _ framework.FilterPlugin = &FilterPlugin{}
var _ framework.PreFilterPlugin = &FilterPlugin{}

// NewFilterPlugin 创建过滤插件
func NewFilterPlugin(c client.Client) *FilterPlugin {
	return &FilterPlugin{
		client: c,
	}
}

// Name 返回插件名称
func (p *FilterPlugin) Name() string {
	return PluginName
}

// PreFilter 预过滤阶段
func (p *FilterPlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	// 解析 Pod 的算力请求
	req := p.parseComputeRequest(pod)
	if req == nil {
		// 没有 HCS 资源请求，跳过
		return framework.NewStatus(framework.Success, "")
	}

	// 将请求存储到 CycleState 供 Filter 阶段使用
	state.Write(stateKey, &stateData{request: req})

	return framework.NewStatus(framework.Success, "")
}

// Filter 过滤阶段
func (p *FilterPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	// 获取预过滤阶段存储的请求
	data, err := state.Read(stateKey)
	if err != nil {
		// 没有 HCS 资源请求，放行
		return framework.NewStatus(framework.Success, "")
	}

	req := data.(*stateData).request

	// 获取节点对应的 ComputeNode
	cn, err := p.getComputeNode(ctx, nodeInfo.Node().Name)
	if err != nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable,
			fmt.Sprintf("failed to get ComputeNode: %v", err))
	}

	if cn == nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable,
			"node does not have ComputeNode resource")
	}

	// 检查节点是否就绪
	if cn.Status.Phase != v1alpha1.ComputeNodePhaseReady {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable,
			fmt.Sprintf("ComputeNode is not ready: %s", cn.Status.Phase))
	}

	// 检查 VRAM 是否满足
	if req.VRAMBytes > 0 {
		availableVRAM := p.calculateAvailableVRAM(cn)
		if availableVRAM < req.VRAMBytes {
			return framework.NewStatus(framework.Unschedulable,
				fmt.Sprintf("insufficient VRAM: requested %d, available %d",
					req.VRAMBytes, availableVRAM))
		}
	}

	// 检查算力是否满足
	if req.FP16TFLOPS > 0 {
		if cn.Spec.TotalCapacity.FP16TFLOPS < req.FP16TFLOPS {
			return framework.NewStatus(framework.Unschedulable,
				fmt.Sprintf("insufficient FP16 TFLOPS: requested %d, available %d",
					req.FP16TFLOPS, cn.Spec.TotalCapacity.FP16TFLOPS))
		}
	}

	return framework.NewStatus(framework.Success, "")
}

// parseComputeRequest 解析 Pod 的算力请求
func (p *FilterPlugin) parseComputeRequest(pod *v1.Pod) *ComputeRequest {
	req := &ComputeRequest{}
	hasRequest := false

	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests == nil {
			continue
		}

		// 解析 VRAM 请求
		if vram, ok := container.Resources.Requests[v1.ResourceName(VRAMResourceName)]; ok {
			req.VRAMBytes += uint64(vram.Value())
			hasRequest = true
		}

		// 解析 FP16 TFLOPS 请求
		if fp16, ok := container.Resources.Requests[v1.ResourceName(FP16TFLOPSResourceName)]; ok {
			req.FP16TFLOPS += uint64(fp16.Value())
			hasRequest = true
		}
	}

	if !hasRequest {
		return nil
	}

	return req
}

// getComputeNode 获取节点对应的 ComputeNode
func (p *FilterPlugin) getComputeNode(ctx context.Context, nodeName string) (*v1alpha1.ComputeNode, error) {
	if p.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	cn := &v1alpha1.ComputeNode{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: nodeName}, cn); err != nil {
		return nil, err
	}

	return cn, nil
}

// calculateAvailableVRAM 计算可用 VRAM
func (p *FilterPlugin) calculateAvailableVRAM(cn *v1alpha1.ComputeNode) uint64 {
	var totalUsed uint64
	for _, dev := range cn.Status.Devices {
		totalUsed += dev.VRAMUsed
	}
	if cn.Spec.TotalCapacity.VRAM > totalUsed {
		return cn.Spec.TotalCapacity.VRAM - totalUsed
	}
	return 0
}

// ComputeRequest 算力请求
type ComputeRequest struct {
	VRAMBytes  uint64
	FP16TFLOPS uint64
	FP32TFLOPS uint64
}

// stateKey CycleState 键
const stateKey = "HCSComputeFilter"

// stateData CycleState 数据
type stateData struct {
	request *ComputeRequest
}

// Clone 克隆状态数据
func (d *stateData) Clone() framework.StateData {
	return &stateData{
		request: &ComputeRequest{
			VRAMBytes:  d.request.VRAMBytes,
			FP16TFLOPS: d.request.FP16TFLOPS,
			FP32TFLOPS: d.request.FP32TFLOPS,
		},
	}
}
