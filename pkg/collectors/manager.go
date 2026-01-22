package collectors

import (
	"context"
	"sync"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// Manager 采集器管理器
type Manager struct {
	collectors []Collector
	mu         sync.RWMutex
}

// NewManager 创建采集器管理器
func NewManager() *Manager {
	return &Manager{
		collectors: make([]Collector, 0),
	}
}

// NewDefaultManager 创建带有默认采集器的管理器
func NewDefaultManager() *Manager {
	m := NewManager()
	m.Register(NewFingerprintCollector())
	m.Register(NewHealthCollector())
	m.Register(NewTopologyCollector())
	return m
}

// Register 注册采集器
func (m *Manager) Register(collector Collector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collectors = append(m.collectors, collector)
}

// CollectAll 运行所有采集器并合并结果
func (m *Manager) CollectAll(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) (*Metrics, error) {
	m.mu.RLock()
	collectors := make([]Collector, len(m.collectors))
	copy(collectors, m.collectors)
	m.mu.RUnlock()

	result := &Metrics{
		DeviceMetrics: make([]DeviceMetric, 0, len(devices)),
	}

	// 初始化每个设备的指标
	deviceMetrics := make(map[string]*DeviceMetric)
	for _, dev := range devices {
		deviceMetrics[dev.ID] = &DeviceMetric{
			DeviceID: dev.ID,
		}
	}

	// 运行每个采集器
	for _, collector := range collectors {
		metrics, err := collector.Collect(ctx, devices, topology)
		if err != nil {
			continue // 单个采集器失败不影响其他采集器
		}

		// 合并结果
		switch collector.Name() {
		case "fingerprint":
			result.Fingerprint = metrics.Fingerprint
			for _, dm := range metrics.DeviceMetrics {
				if existing, ok := deviceMetrics[dm.DeviceID]; ok {
					existing.Fingerprint = dm.Fingerprint
				}
			}
		case "health":
			result.Health = metrics.Health
			for _, dm := range metrics.DeviceMetrics {
				if existing, ok := deviceMetrics[dm.DeviceID]; ok {
					existing.Health = dm.Health
				}
			}
		case "topology":
			result.Topology = metrics.Topology
		}
	}

	// 构建设备指标列表
	for _, dev := range devices {
		if dm, ok := deviceMetrics[dev.ID]; ok {
			result.DeviceMetrics = append(result.DeviceMetrics, *dm)
		}
	}

	return result, nil
}

// List 列出所有已注册的采集器
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.collectors))
	for _, c := range m.collectors {
		names = append(names, c.Name())
	}
	return names
}

// CollectorResult 单个采集器的结果
type CollectorResult struct {
	Name    string
	Metrics *Metrics
	Error   error
}

// CollectAllParallel 并行运行所有采集器
func (m *Manager) CollectAllParallel(ctx context.Context, devices []*detectors.Device, topology *detectors.Topology) ([]*CollectorResult, error) {
	m.mu.RLock()
	collectors := make([]Collector, len(m.collectors))
	copy(collectors, m.collectors)
	m.mu.RUnlock()

	results := make([]*CollectorResult, len(collectors))
	var wg sync.WaitGroup

	for i, collector := range collectors {
		wg.Add(1)
		go func(idx int, c Collector) {
			defer wg.Done()
			metrics, err := c.Collect(ctx, devices, topology)
			results[idx] = &CollectorResult{
				Name:    c.Name(),
				Metrics: metrics,
				Error:   err,
			}
		}(i, collector)
	}

	wg.Wait()
	return results, nil
}
