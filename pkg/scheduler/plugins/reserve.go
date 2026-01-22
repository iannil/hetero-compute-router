package plugins

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/framework"
)

const (
	// ReservePluginName 预留插件名称
	ReservePluginName = "HCSComputeReserve"
)

// ReservePlugin HCS 资源预留插件
type ReservePlugin struct {
	client client.Client

	// reservations 存储每个节点的资源预留
	// key: nodeName, value: 预留的资源量
	reservations map[string]*Reservation
	mu           sync.RWMutex
}

// Reservation 资源预留
type Reservation struct {
	// Pods 存储在该节点预留的 Pod
	Pods map[string]*PodReservation
}

// PodReservation Pod 资源预留
type PodReservation struct {
	VRAMBytes  uint64
	FP16TFLOPS uint64
}

var _ framework.ReservePlugin = &ReservePlugin{}

// NewReservePlugin 创建预留插件
func NewReservePlugin(c client.Client) *ReservePlugin {
	return &ReservePlugin{
		client:       c,
		reservations: make(map[string]*Reservation),
	}
}

// Name 返回插件名称
func (p *ReservePlugin) Name() string {
	return ReservePluginName
}

// Reserve 预留资源
func (p *ReservePlugin) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	// 获取请求数据
	data, err := state.Read(stateKey)
	if err != nil {
		// 没有 HCS 资源请求，不需要预留
		return framework.NewStatus(framework.Success, "")
	}

	req := data.(*stateData).request
	podKey := getPodKey(pod)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取或创建节点的预留记录
	reservation, ok := p.reservations[nodeName]
	if !ok {
		reservation = &Reservation{
			Pods: make(map[string]*PodReservation),
		}
		p.reservations[nodeName] = reservation
	}

	// 检查是否已存在预留（幂等性）
	if _, exists := reservation.Pods[podKey]; exists {
		klog.V(2).Infof("Reservation already exists for pod %s on node %s", podKey, nodeName)
		return framework.NewStatus(framework.Success, "")
	}

	// 验证资源是否足够（包含已预留的资源）
	cn, err := p.getComputeNode(ctx, nodeName)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("failed to get ComputeNode: %v", err))
	}

	availableVRAM := p.calculateAvailableVRAMWithReservation(cn, nodeName)
	if req.VRAMBytes > 0 && availableVRAM < req.VRAMBytes {
		return framework.NewStatus(framework.Unschedulable,
			fmt.Sprintf("insufficient VRAM after reservation: requested %d, available %d",
				req.VRAMBytes, availableVRAM))
	}

	// 创建预留
	reservation.Pods[podKey] = &PodReservation{
		VRAMBytes:  req.VRAMBytes,
		FP16TFLOPS: req.FP16TFLOPS,
	}

	klog.V(2).Infof("Reserved resources for pod %s on node %s: VRAM=%d, FP16TFLOPS=%d",
		podKey, nodeName, req.VRAMBytes, req.FP16TFLOPS)

	return framework.NewStatus(framework.Success, "")
}

// Unreserve 取消资源预留
func (p *ReservePlugin) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	podKey := getPodKey(pod)

	p.mu.Lock()
	defer p.mu.Unlock()

	reservation, ok := p.reservations[nodeName]
	if !ok {
		return
	}

	if _, exists := reservation.Pods[podKey]; exists {
		delete(reservation.Pods, podKey)
		klog.V(2).Infof("Unreserved resources for pod %s on node %s", podKey, nodeName)
	}

	// 如果节点没有任何预留，清理记录
	if len(reservation.Pods) == 0 {
		delete(p.reservations, nodeName)
	}
}

// calculateAvailableVRAMWithReservation 计算考虑预留后的可用 VRAM
func (p *ReservePlugin) calculateAvailableVRAMWithReservation(cn *v1alpha1.ComputeNode, nodeName string) uint64 {
	// 计算实际使用量
	var totalUsed uint64
	for _, dev := range cn.Status.Devices {
		totalUsed += dev.VRAMUsed
	}

	// 加上已预留的量
	if reservation, ok := p.reservations[nodeName]; ok {
		for _, podRes := range reservation.Pods {
			totalUsed += podRes.VRAMBytes
		}
	}

	if cn.Spec.TotalCapacity.VRAM > totalUsed {
		return cn.Spec.TotalCapacity.VRAM - totalUsed
	}
	return 0
}

// GetTotalReservedVRAM 获取节点上预留的总 VRAM
func (p *ReservePlugin) GetTotalReservedVRAM(nodeName string) uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var total uint64
	if reservation, ok := p.reservations[nodeName]; ok {
		for _, podRes := range reservation.Pods {
			total += podRes.VRAMBytes
		}
	}
	return total
}

// ClearPodReservation 清除指定 Pod 的预留（Pod 启动成功后调用）
func (p *ReservePlugin) ClearPodReservation(podKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for nodeName, reservation := range p.reservations {
		if _, exists := reservation.Pods[podKey]; exists {
			delete(reservation.Pods, podKey)
			klog.V(2).Infof("Cleared reservation for pod %s on node %s", podKey, nodeName)

			if len(reservation.Pods) == 0 {
				delete(p.reservations, nodeName)
			}
			return
		}
	}
}

// getComputeNode 获取 ComputeNode
func (p *ReservePlugin) getComputeNode(ctx context.Context, nodeName string) (*v1alpha1.ComputeNode, error) {
	if p.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	cn := &v1alpha1.ComputeNode{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: nodeName}, cn); err != nil {
		return nil, err
	}

	return cn, nil
}

// getPodKey 获取 Pod 唯一键
func getPodKey(pod *v1.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}
