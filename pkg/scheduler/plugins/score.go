package plugins

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/zrs-products/hetero-compute-router/pkg/api/v1alpha1"
	"github.com/zrs-products/hetero-compute-router/pkg/exchange"
	"github.com/zrs-products/hetero-compute-router/pkg/scheduler/framework"
)

const (
	// ScorePluginName 打分插件名称
	ScorePluginName = "HCSComputeScore"

	// MaxScore 最高分
	MaxScore int64 = 100
)

// ScorePlugin HCS 打分插件
type ScorePlugin struct {
	client     client.Client
	calculator *exchange.Calculator
}

var _ framework.ScorePlugin = &ScorePlugin{}
var _ framework.ScoreExtensions = &ScorePlugin{}

// NewScorePlugin 创建打分插件
func NewScorePlugin(c client.Client, calc *exchange.Calculator) *ScorePlugin {
	if calc == nil {
		calc = exchange.NewCalculator()
	}
	return &ScorePlugin{
		client:     c,
		calculator: calc,
	}
}

// Name 返回插件名称
func (p *ScorePlugin) Name() string {
	return ScorePluginName
}

// Score 计算节点分数
func (p *ScorePlugin) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	// 获取请求数据
	data, err := state.Read(stateKey)
	if err != nil {
		// 没有 HCS 资源请求，返回默认分数
		return MaxScore / 2, framework.NewStatus(framework.Success, "")
	}

	req := data.(*stateData).request

	// 获取 ComputeNode
	cn, err := p.getComputeNode(ctx, nodeName)
	if err != nil || cn == nil {
		return 0, framework.NewStatus(framework.Success, "")
	}

	// 计算综合分数
	score := p.calculateScore(cn, req)

	return score, framework.NewStatus(framework.Success, "")
}

// ScoreExtensions 返回打分扩展
func (p *ScorePlugin) ScoreExtensions() framework.ScoreExtensions {
	return p
}

// NormalizeScore 归一化分数
func (p *ScorePlugin) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// 找出最高分和最低分
	var maxScore, minScore int64 = 0, MaxScore
	for _, score := range scores {
		if score.Score > maxScore {
			maxScore = score.Score
		}
		if score.Score < minScore {
			minScore = score.Score
		}
	}

	// 归一化到 0-100
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		for i := range scores {
			scores[i].Score = MaxScore / 2
		}
		return framework.NewStatus(framework.Success, "")
	}

	for i := range scores {
		scores[i].Score = (scores[i].Score - minScore) * MaxScore / scoreRange
	}

	return framework.NewStatus(framework.Success, "")
}

// calculateScore 计算节点分数
func (p *ScorePlugin) calculateScore(cn *v1alpha1.ComputeNode, req *ComputeRequest) int64 {
	var score int64 = 0

	// 获取节点硬件信息
	vendor := cn.Spec.Vendor
	model := p.getDeviceModel(cn)
	deviceCount := len(cn.Status.Devices)

	// 1. 使用汇率归一化计算分数（40%权重）
	// 归一化后的分数反映跨厂商的等效算力
	if vendor != "" && model != "" && deviceCount > 0 {
		normalized, err := p.calculator.NormalizeCompute(vendor, model, deviceCount)
		if err == nil {
			// 归一化算力分数：综合 TFLOPS 和 VRAM
			computeRatio := normalized.NormalizedTFLOPS
			if computeRatio > 1.0 {
				computeRatio = 1.0 // 上限为 1.0
			}
			score += int64(computeRatio * 25) // 25% for normalized compute

			memoryRatio := normalized.NormalizedVRAM
			if memoryRatio > 1.0 {
				memoryRatio = 1.0
			}
			score += int64(memoryRatio * 15) // 15% for normalized memory
		} else {
			// 归一化失败，使用原始 VRAM 比例作为后备
			availableVRAM := p.calculateAvailableVRAM(cn)
			totalVRAM := cn.Spec.TotalCapacity.VRAM
			if totalVRAM > 0 {
				vramRatio := float64(availableVRAM) / float64(totalVRAM)
				score += int64(vramRatio * 40)
			}
		}
	} else {
		// 没有硬件信息，使用原始 VRAM 余量
		availableVRAM := p.calculateAvailableVRAM(cn)
		totalVRAM := cn.Spec.TotalCapacity.VRAM
		if totalVRAM > 0 {
			vramRatio := float64(availableVRAM) / float64(totalVRAM)
			score += int64(vramRatio * 40)
		}
	}

	// 2. 健康分数（30%权重）
	avgHealth := p.calculateAverageHealth(cn)
	score += int64(avgHealth * 0.3)

	// 3. 算力匹配分数（20%权重）
	// 优先选择算力更匹配的节点，避免资源浪费
	if req.FP16TFLOPS > 0 && cn.Spec.TotalCapacity.FP16TFLOPS > 0 {
		matchRatio := float64(req.FP16TFLOPS) / float64(cn.Spec.TotalCapacity.FP16TFLOPS)
		// 匹配度越接近 1（不超过1），分数越高
		if matchRatio <= 1 {
			score += int64(matchRatio * 20)
		}
	} else {
		score += 10 // 没有特定算力需求，给基础分
	}

	// 4. 互联类型加分（10%权重）
	// NVLink 节点优先
	for _, dev := range cn.Status.Devices {
		if dev.InterconnectType == "NVLink" {
			score += 10
			break
		}
	}

	return score
}

// getDeviceModel 从 ComputeNode 获取设备型号
func (p *ScorePlugin) getDeviceModel(cn *v1alpha1.ComputeNode) string {
	if len(cn.Status.Devices) > 0 {
		return cn.Status.Devices[0].Model
	}
	return ""
}

// calculateAvailableVRAM 计算可用 VRAM
func (p *ScorePlugin) calculateAvailableVRAM(cn *v1alpha1.ComputeNode) uint64 {
	var totalUsed uint64
	for _, dev := range cn.Status.Devices {
		totalUsed += dev.VRAMUsed
	}
	if cn.Spec.TotalCapacity.VRAM > totalUsed {
		return cn.Spec.TotalCapacity.VRAM - totalUsed
	}
	return 0
}

// calculateAverageHealth 计算平均健康分
func (p *ScorePlugin) calculateAverageHealth(cn *v1alpha1.ComputeNode) float64 {
	if len(cn.Status.Devices) == 0 {
		return 100.0
	}

	var total float64
	for _, dev := range cn.Status.Devices {
		total += dev.HealthScore
	}
	return total / float64(len(cn.Status.Devices))
}

// getComputeNode 获取 ComputeNode
func (p *ScorePlugin) getComputeNode(ctx context.Context, nodeName string) (*v1alpha1.ComputeNode, error) {
	if p.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	cn := &v1alpha1.ComputeNode{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: nodeName}, cn); err != nil {
		return nil, err
	}

	return cn, nil
}
