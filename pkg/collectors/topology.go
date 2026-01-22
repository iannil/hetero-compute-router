package collectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// TopologyCollector 拓扑信息采集器
type TopologyCollector struct{}

// NewTopologyCollector 创建拓扑采集器
func NewTopologyCollector() *TopologyCollector {
	return &TopologyCollector{}
}

// Name 返回采集器名称
func (c *TopologyCollector) Name() string {
	return "topology"
}

// Collect 采集拓扑信息
func (c *TopologyCollector) Collect(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*Metrics, error) {
	if len(devices) == 0 {
		return &Metrics{
			Topology: &TopologyMetrics{},
		}, nil
	}

	metrics := &Metrics{
		Topology: &TopologyMetrics{
			Peers: make([]PeerInfo, 0),
		},
	}

	// 构建设备 ID 到索引的映射
	deviceIndex := make(map[string]int)
	for i, dev := range devices {
		deviceIndex[dev.ID] = i
	}

	// 从检测器拓扑信息构建 PeerInfo
	if topology != nil {
		for _, link := range topology.Links {
			// 添加双向连接
			metrics.Topology.Peers = append(metrics.Topology.Peers, PeerInfo{
				DeviceID: link.TargetID,
				LinkType: string(link.Type),
				LinkBw:   link.Bandwidth,
				Hops:     1, // 直连为 1 跳
			})
		}
	}

	// 解析第一个设备的 PCIe 信息作为节点级拓扑信息
	if len(devices) > 0 && devices[0].PCIEBusID != "" {
		pcie := c.parsePCIEBusID(devices[0].PCIEBusID)
		metrics.Topology.PCIEDomain = pcie.Domain
		metrics.Topology.PCIEBus = pcie.Bus
		metrics.Topology.PCIEDevice = pcie.Device
		metrics.Topology.PCIEFunction = pcie.Function
	}

	return metrics, nil
}

// PCIEInfo PCIe 总线信息
type PCIEInfo struct {
	Domain   uint32
	Bus      uint32
	Device   uint32
	Function uint32
}

// parsePCIEBusID 解析 PCIe 总线 ID (格式: DDDD:BB:DD.F)
func (c *TopologyCollector) parsePCIEBusID(busID string) PCIEInfo {
	info := PCIEInfo{}

	// 格式: DDDD:BB:DD.F 或 BB:DD.F
	parts := strings.Split(busID, ":")
	if len(parts) < 2 {
		return info
	}

	var domainStr, busStr, deviceFuncStr string
	if len(parts) == 3 {
		domainStr = parts[0]
		busStr = parts[1]
		deviceFuncStr = parts[2]
	} else {
		domainStr = "0000"
		busStr = parts[0]
		deviceFuncStr = parts[1]
	}

	// 解析 domain
	if domain, err := strconv.ParseUint(domainStr, 16, 32); err == nil {
		info.Domain = uint32(domain)
	}

	// 解析 bus
	if bus, err := strconv.ParseUint(busStr, 16, 32); err == nil {
		info.Bus = uint32(bus)
	}

	// 解析 device.function
	dfParts := strings.Split(deviceFuncStr, ".")
	if len(dfParts) >= 1 {
		if device, err := strconv.ParseUint(dfParts[0], 16, 32); err == nil {
			info.Device = uint32(device)
		}
	}
	if len(dfParts) >= 2 {
		if function, err := strconv.ParseUint(dfParts[1], 16, 32); err == nil {
			info.Function = uint32(function)
		}
	}

	return info
}

// BuildTopologyMatrix 构建拓扑矩阵（用于调度决策）
func (c *TopologyCollector) BuildTopologyMatrix(devices []*detectors.Device, topology *detectors.Topology) [][]int {
	n := len(devices)
	if n == 0 {
		return nil
	}

	// 创建邻接矩阵，值表示连接带宽（0 表示无连接）
	matrix := make([][]int, n)
	for i := range matrix {
		matrix[i] = make([]int, n)
	}

	// 构建设备 ID 到索引的映射
	deviceIndex := make(map[string]int)
	for i, dev := range devices {
		deviceIndex[dev.ID] = i
	}

	// 填充矩阵
	if topology != nil {
		for _, link := range topology.Links {
			srcIdx, srcOk := deviceIndex[link.SourceID]
			tgtIdx, tgtOk := deviceIndex[link.TargetID]
			if srcOk && tgtOk {
				bw := int(link.Bandwidth)
				matrix[srcIdx][tgtIdx] = bw
				matrix[tgtIdx][srcIdx] = bw // 对称
			}
		}
	}

	return matrix
}

// FindOptimalPlacement 找到最优的设备放置（基于拓扑亲和性）
func (c *TopologyCollector) FindOptimalPlacement(matrix [][]int, requiredDevices int) []int {
	n := len(matrix)
	if requiredDevices > n || requiredDevices <= 0 {
		return nil
	}

	if requiredDevices == 1 {
		return []int{0}
	}

	// 简单贪心算法：找到互联带宽总和最大的设备组合
	bestPlacement := make([]int, requiredDevices)
	bestScore := -1

	// 对于小规模问题使用暴力搜索，大规模问题使用贪心
	if n <= 8 && requiredDevices <= 4 {
		c.findBestCombination(matrix, requiredDevices, 0, make([]int, 0, requiredDevices), &bestPlacement, &bestScore)
	} else {
		// 贪心方法：从第一个设备开始，依次选择与已选设备互联带宽最大的设备
		selected := make([]bool, n)
		placement := make([]int, 0, requiredDevices)

		// 选择第一个设备（带宽总和最大的）
		maxSum := -1
		firstDev := 0
		for i := 0; i < n; i++ {
			sum := 0
			for j := 0; j < n; j++ {
				sum += matrix[i][j]
			}
			if sum > maxSum {
				maxSum = sum
				firstDev = i
			}
		}
		placement = append(placement, firstDev)
		selected[firstDev] = true

		// 依次选择剩余设备
		for len(placement) < requiredDevices {
			maxScore := -1
			nextDev := -1
			for i := 0; i < n; i++ {
				if selected[i] {
					continue
				}
				score := 0
				for _, dev := range placement {
					score += matrix[i][dev]
				}
				if score > maxScore {
					maxScore = score
					nextDev = i
				}
			}
			if nextDev >= 0 {
				placement = append(placement, nextDev)
				selected[nextDev] = true
			}
		}
		bestPlacement = placement
	}

	return bestPlacement
}

// findBestCombination 递归查找最佳组合
func (c *TopologyCollector) findBestCombination(matrix [][]int, k, start int, current []int, best *[]int, bestScore *int) {
	if len(current) == k {
		score := c.calculatePlacementScore(matrix, current)
		if score > *bestScore {
			*bestScore = score
			copy(*best, current)
		}
		return
	}

	for i := start; i < len(matrix); i++ {
		c.findBestCombination(matrix, k, i+1, append(current, i), best, bestScore)
	}
}

// calculatePlacementScore 计算放置方案的得分（互联带宽总和）
func (c *TopologyCollector) calculatePlacementScore(matrix [][]int, placement []int) int {
	score := 0
	for i := 0; i < len(placement); i++ {
		for j := i + 1; j < len(placement); j++ {
			score += matrix[placement[i]][placement[j]]
		}
	}
	return score
}

// String 返回拓扑的字符串表示
func (t *TopologyMetrics) String() string {
	return fmt.Sprintf("PCIe[%04x:%02x:%02x.%x] Peers=%d",
		t.PCIEDomain, t.PCIEBus, t.PCIEDevice, t.PCIEFunction, len(t.Peers))
}

// Ensure TopologyCollector implements Collector interface
var _ Collector = (*TopologyCollector)(nil)
