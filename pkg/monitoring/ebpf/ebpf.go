package ebpf

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// EBPFManager eBPF 监控管理器
// 负责加载 eBPF 程序、附加到 tracepoints、处理事件
type EBPFManager struct {
	mu sync.RWMutex

	// eBPF 程序状态
	initialized bool
	enabled     bool

	// 事件通道
	gpuEvents    chan GPUEvent
	pcieEvents   chan PCIeEvent
	healthEvents chan HealthEvent

	// 最新快照
	snapshots map[uint32]*DeviceHealthSnapshot

	// 分析器
	analyzer *HealthAnalyzer

	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	config *Config
}

// Config eBPF 管理器配置
type Config struct {
	// 是否启用 eBPF 监控
	Enabled bool
	// GPU 事件采样间隔
	GPUSampleInterval time.Duration
	// PCIe 事件采样间隔
	PCIeSampleInterval time.Duration
	// 缓冲区大小
	BufferSize int
	// 是否在 eBPF 不可用时回退到轮询模式
	FallbackToPolling bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		GPUSampleInterval: 100 * time.Millisecond,
		PCIeSampleInterval: 100 * time.Millisecond,
		BufferSize:        1000,
		FallbackToPolling: true,
	}
}

// NewEBPFManager 创建新的 eBPF 管理器
func NewEBPFManager(config *Config) (*EBPFManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	m := &EBPFManager{
		enabled:     config.Enabled,
		gpuEvents:   make(chan GPUEvent, config.BufferSize),
		pcieEvents:  make(chan PCIeEvent, config.BufferSize),
		healthEvents: make(chan HealthEvent, config.BufferSize),
		snapshots:   make(map[uint32]*DeviceHealthSnapshot),
		analyzer:    NewHealthAnalyzer(),
		config:      config,
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())

	// 检查是否支持 eBPF
	if !m.checkBPFSupport() {
		if config.FallbackToPolling {
			// 回退到轮询模式
			m.enabled = false
		} else {
			return nil, fmt.Errorf("eBPF not supported and fallback disabled")
		}
	}

	return m, nil
}

// checkBPFSupport 检查系统是否支持 eBPF
func (m *EBPFManager) checkBPFSupport() bool {
	// 检查 /sys/kernel/debug/tracing 是否存在
	if _, err := os.Stat("/sys/kernel/debug/tracing"); os.IsNotExist(err) {
		return false
	}

	// 检查 BPF 文件系统
	if _, err := os.Stat("/sys/fs/bpf"); os.IsNotExist(err) {
		return false
	}

	return true
}

// Start 启动 eBPF 监控
func (m *EBPFManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// 尝试加载 eBPF 程序
	if m.enabled {
		if err := m.loadPrograms(); err != nil {
			if m.config.FallbackToPolling {
				m.enabled = false
			} else {
				return fmt.Errorf("failed to load eBPF programs: %w", err)
			}
		}
	}

	// 启动事件处理器
	m.wg.Add(3)
	go m.processGPUEvents()
	go m.processPCIeEvents()
	go m.processHealthEvents()

	m.initialized = true
	return nil
}

// loadPrograms 加载 eBPF 程序
func (m *EBPFManager) loadPrograms() error {
	// 这里应该编译和加载 eBPF 程序
	// 由于需要 clang 和特定的内核头文件，这里只做框架

	// 实际实现会:
	// 1. 编译 .c 文件为 .o
	// 2. 使用 cilium/ebpf 加载
	// 3. 附加到 tracepoints

	// 目前返回错误，触发回退
	return fmt.Errorf("eBPF programs not compiled")
}

// attachTracepoints 附加到内核 tracepoints
func (m *EBPFManager) attachTracepoints() error {
	// 附加到 NVIDIA GPU tracepoints (如果存在)
	// /sys/kernel/debug/tracing/events/nvml/

	// 附加到 AMD GPU tracepoints (如果存在)
	// /sys/kernel/debug/tracing/events/amdgpu/

	// 附加到 PCIe tracepoints
	// /sys/kernel/debug/tracing/events/irq/

	return nil
}

// processGPUEvents 处理 GPU 事件
func (m *EBPFManager) processGPUEvents() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.GPUSampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.gpuEvents:
			m.analyzer.AddGPUEvent(event)
		case <-ticker.C:
			if !m.enabled {
				continue
			}
			// 从 eBPF map 读取数据
			m.readGPUMap()
		}
	}
}

// processPCIeEvents 处理 PCIe 事件
func (m *EBPFManager) processPCIeEvents() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.PCIeSampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.pcieEvents:
			m.analyzer.AddPCIeEvent(event)
		case <-ticker.C:
			if !m.enabled {
				continue
			}
			// 从 eBPF map 读取数据
			m.readPCIeMap()
		}
	}
}

// processHealthEvents 处理健康事件
func (m *EBPFManager) processHealthEvents() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.healthEvents:
			m.analyzer.AddHealthEvent(event)
		}
	}
}

// readGPUMap 从 eBPF map 读取 GPU 数据
func (m *EBPFManager) readGPUMap() {
	// 实际实现会从 BPF map 读取数据
	// 这里只是框架
}

// readPCIeMap 从 eBPF map 读取 PCIe 数据
func (m *EBPFManager) readPCIeMap() {
	// 实际实现会从 BPF map 读取数据
	// 这里只是框架
}

// GetSnapshot 获取设备健康快照
func (m *EBPFManager) GetSnapshot(deviceID uint32) *DeviceHealthSnapshot {
	return m.analyzer.GetSnapshot(deviceID)
}

// GetAllSnapshots 获取所有设备快照
func (m *EBPFManager) GetAllSnapshots() map[uint32]*DeviceHealthSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[uint32]*DeviceHealthSnapshot)
	for id := range m.snapshots {
		result[id] = m.GetSnapshot(id)
	}
	return result
}

// IsPredictiveFailure 检查设备是否预测性故障
func (m *EBPFManager) IsPredictiveFailure(deviceID uint32) (bool, string) {
	return m.analyzer.IsPredictiveFailure(deviceID)
}

// AddDevices 添加要监控的设备
func (m *EBPFManager) AddDevices(devices []*detectors.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, dev := range devices {
		// 解析设备 ID
		var deviceID uint32
		fmt.Sscanf(dev.ID, "gpu-%d", &deviceID)
		fmt.Sscanf(dev.ID, "dcu-%d", &deviceID)

		// 初始化快照
		if _, ok := m.snapshots[deviceID]; !ok {
			m.snapshots[deviceID] = &DeviceHealthSnapshot{
				DeviceID:    deviceID,
				Temperature: dev.Temperature,
				Power:       dev.PowerUsage,
				HealthScore: dev.HealthScore,
			}
		}
	}
}

// Stop 停止 eBPF 监控
func (m *EBPFManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.initialized {
		return nil
	}

	m.cancel()
	m.wg.Wait()

	// 卸载 eBPF 程序
	if m.enabled {
		m.unloadPrograms()
	}

	m.initialized = false
	return nil
}

// unloadPrograms 卸载 eBPF 程序
func (m *EBPFManager) unloadPrograms() error {
	// 实际实现会分离 tracepoints 并关闭程序
	return nil
}

// IsEnabled 返回是否启用了 eBPF
func (m *EBPFManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetAnalyzer 获取分析器（用于 Collector 集成）
func (m *EBPFManager) GetAnalyzer() *HealthAnalyzer {
	return m.analyzer
}
