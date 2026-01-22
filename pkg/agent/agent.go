package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/zrs-products/hetero-compute-router/pkg/collectors"
	"github.com/zrs-products/hetero-compute-router/pkg/detectors"
)

// Config Agent 配置
type Config struct {
	// NodeName Kubernetes 节点名称
	NodeName string

	// CollectInterval 采集间隔
	CollectInterval time.Duration

	// ReportInterval 上报间隔
	ReportInterval time.Duration

	// UseMock 是否使用 Mock 检测器
	UseMock bool

	// MockConfig Mock 检测器配置
	MockConfig *MockConfig
}

// MockConfig Mock 配置
type MockConfig struct {
	DeviceCount int
	GPUModel    string
	VRAMPerGPU  uint64
	HasNVLink   bool
}

// DefaultConfig 默认配置
func DefaultConfig(nodeName string) *Config {
	return &Config{
		NodeName:        nodeName,
		CollectInterval: 10 * time.Second,
		ReportInterval:  30 * time.Second,
		UseMock:         false,
	}
}

// Agent Node-Agent 核心结构
type Agent struct {
	config     *Config
	detector   detectors.Detector
	collectors *collectors.Manager
	reporter   *Reporter

	// 最新采集的数据
	latestMetrics *collectors.Metrics
	latestDevices []*detectors.Device
	latestHWType  *detectors.HardwareType
	mu            sync.RWMutex

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// New 创建 Agent
func New(config *Config, reporter *Reporter) (*Agent, error) {
	if config.NodeName == "" {
		return nil, fmt.Errorf("node name is required")
	}

	agent := &Agent{
		config:     config,
		collectors: collectors.NewDefaultManager(),
		reporter:   reporter,
		stopCh:     make(chan struct{}),
	}

	// 初始化检测器
	if err := agent.initDetector(); err != nil {
		return nil, fmt.Errorf("failed to initialize detector: %w", err)
	}

	return agent, nil
}

// initDetector 初始化检测器
func (a *Agent) initDetector() error {
	if a.config.UseMock {
		// 使用 Mock 检测器
		mockConfig := a.config.MockConfig
		if mockConfig == nil {
			mockConfig = &MockConfig{
				DeviceCount: 4,
				GPUModel:    "NVIDIA A100-SXM4-80GB",
				VRAMPerGPU:  80 * 1024 * 1024 * 1024,
				HasNVLink:   true,
			}
		}
		a.detector = newMockDetector(mockConfig)
		klog.Info("Using mock detector")
		return nil
	}

	// 尝试使用真实 NVML 检测器
	registry := detectors.NewRegistry()

	// 注册 NVIDIA 检测器
	nvmlDetector := newNVMLDetector()
	registry.Register(nvmlDetector)

	// 尝试找到可用的检测器
	detector, err := registry.FindAvailable(context.Background())
	if err != nil {
		klog.Warningf("No hardware detected, falling back to mock: %v", err)
		// 回退到 Mock 模式
		a.detector = newMockDetector(&MockConfig{
			DeviceCount: 1,
			GPUModel:    "NVIDIA Virtual GPU",
			VRAMPerGPU:  16 * 1024 * 1024 * 1024,
			HasNVLink:   false,
		})
		return nil
	}

	a.detector = detector
	klog.Infof("Using detector: %s", detector.Name())
	return nil
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	klog.Infof("Starting Node-Agent for node: %s", a.config.NodeName)

	// 执行初始采集
	if err := a.collect(ctx); err != nil {
		klog.Warningf("Initial collection failed: %v", err)
	}

	// 执行初始上报
	if a.reporter != nil {
		if err := a.report(ctx); err != nil {
			klog.Warningf("Initial report failed: %v", err)
		}
	}

	// 启动采集循环
	a.wg.Add(1)
	go a.collectLoop(ctx)

	// 启动上报循环
	if a.reporter != nil {
		a.wg.Add(1)
		go a.reportLoop(ctx)
	}

	klog.Info("Node-Agent started successfully")
	return nil
}

// Stop 停止 Agent
func (a *Agent) Stop() error {
	klog.Info("Stopping Node-Agent")
	close(a.stopCh)
	a.wg.Wait()

	if a.detector != nil {
		if err := a.detector.Close(); err != nil {
			klog.Warningf("Failed to close detector: %v", err)
		}
	}

	klog.Info("Node-Agent stopped")
	return nil
}

// collectLoop 采集循环
func (a *Agent) collectLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.collect(ctx); err != nil {
				klog.Warningf("Collection failed: %v", err)
			}
		}
	}
}

// reportLoop 上报循环
func (a *Agent) reportLoop(ctx context.Context) {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.report(ctx); err != nil {
				klog.Warningf("Report failed: %v", err)
			}
		}
	}
}

// collect 执行一次采集
func (a *Agent) collect(ctx context.Context) error {
	// 检测硬件类型
	hwType, err := a.detector.Detect(ctx)
	if err != nil {
		return fmt.Errorf("hardware detection failed: %w", err)
	}

	// 获取设备列表
	devices, err := a.detector.GetDevices(ctx)
	if err != nil {
		return fmt.Errorf("get devices failed: %w", err)
	}

	// 获取拓扑
	topology, err := a.detector.GetTopology(ctx)
	if err != nil {
		klog.V(2).Infof("Get topology failed (non-fatal): %v", err)
	}

	// 运行采集器
	metrics, err := a.collectors.CollectAll(ctx, devices, topology)
	if err != nil {
		return fmt.Errorf("metrics collection failed: %w", err)
	}

	// 更新最新数据
	a.mu.Lock()
	a.latestHWType = hwType
	a.latestDevices = devices
	a.latestMetrics = metrics
	a.mu.Unlock()

	klog.V(2).Infof("Collected metrics: %d devices, VRAM total: %d GB",
		len(devices), metrics.Fingerprint.VRAMTotal/(1024*1024*1024))

	return nil
}

// report 执行一次上报
func (a *Agent) report(ctx context.Context) error {
	a.mu.RLock()
	hwType := a.latestHWType
	devices := a.latestDevices
	metrics := a.latestMetrics
	a.mu.RUnlock()

	if hwType == nil || metrics == nil {
		return fmt.Errorf("no data to report")
	}

	return a.reporter.Report(ctx, a.config.NodeName, hwType, devices, metrics)
}

// GetLatestMetrics 获取最新的采集数据
func (a *Agent) GetLatestMetrics() *collectors.Metrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latestMetrics
}

// GetLatestDevices 获取最新的设备列表
func (a *Agent) GetLatestDevices() []*detectors.Device {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latestDevices
}

// GetHardwareType 获取硬件类型
func (a *Agent) GetHardwareType() *detectors.HardwareType {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.latestHWType
}

// newMockDetector 创建 Mock 检测器的包装函数
func newMockDetector(config *MockConfig) detectors.Detector {
	// 延迟导入以避免循环依赖
	return &mockDetectorWrapper{config: config}
}

// newNVMLDetector 创建 NVML 检测器的包装函数
func newNVMLDetector() detectors.Detector {
	return &nvmlDetectorWrapper{}
}

// mockDetectorWrapper Mock 检测器包装
type mockDetectorWrapper struct {
	config   *MockConfig
	detector detectors.Detector
	once     sync.Once
}

func (w *mockDetectorWrapper) init() {
	w.once.Do(func() {
		// 动态导入 nvidia 包中的 MockDetector
		// 为避免循环依赖，这里内联实现
		w.detector = createMockDetector(w.config)
	})
}

func (w *mockDetectorWrapper) Name() string {
	w.init()
	return w.detector.Name()
}

func (w *mockDetectorWrapper) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	w.init()
	return w.detector.Detect(ctx)
}

func (w *mockDetectorWrapper) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	w.init()
	return w.detector.GetDevices(ctx)
}

func (w *mockDetectorWrapper) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	w.init()
	return w.detector.GetTopology(ctx)
}

func (w *mockDetectorWrapper) Close() error {
	if w.detector != nil {
		return w.detector.Close()
	}
	return nil
}

// nvmlDetectorWrapper NVML 检测器包装
type nvmlDetectorWrapper struct {
	detector detectors.Detector
	once     sync.Once
	initErr  error
}

func (w *nvmlDetectorWrapper) init() {
	w.once.Do(func() {
		// 动态创建 NVML 检测器
		w.detector = createNVMLDetector()
	})
}

func (w *nvmlDetectorWrapper) Name() string {
	w.init()
	if w.detector == nil {
		return "nvidia-nvml"
	}
	return w.detector.Name()
}

func (w *nvmlDetectorWrapper) Detect(ctx context.Context) (*detectors.HardwareType, error) {
	w.init()
	if w.detector == nil {
		return nil, fmt.Errorf("NVML detector not initialized")
	}
	return w.detector.Detect(ctx)
}

func (w *nvmlDetectorWrapper) GetDevices(ctx context.Context) ([]*detectors.Device, error) {
	w.init()
	if w.detector == nil {
		return nil, fmt.Errorf("NVML detector not initialized")
	}
	return w.detector.GetDevices(ctx)
}

func (w *nvmlDetectorWrapper) GetTopology(ctx context.Context) (*detectors.Topology, error) {
	w.init()
	if w.detector == nil {
		return nil, fmt.Errorf("NVML detector not initialized")
	}
	return w.detector.GetTopology(ctx)
}

func (w *nvmlDetectorWrapper) Close() error {
	if w.detector != nil {
		return w.detector.Close()
	}
	return nil
}
