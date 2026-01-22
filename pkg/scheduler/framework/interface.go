package framework

import (
	"context"

	v1 "k8s.io/api/core/v1"
)

// Status 表示插件执行结果
type Status struct {
	Code    StatusCode
	Message string
}

// StatusCode 状态码
type StatusCode int

const (
	// Success 成功
	Success StatusCode = iota
	// Unschedulable 可调度但当前不满足条件
	Unschedulable
	// UnschedulableAndUnresolvable 不可调度且无法解决
	UnschedulableAndUnresolvable
	// Error 内部错误
	Error
)

// NewStatus 创建状态
func NewStatus(code StatusCode, message string) *Status {
	return &Status{
		Code:    code,
		Message: message,
	}
}

// IsSuccess 是否成功
func (s *Status) IsSuccess() bool {
	return s == nil || s.Code == Success
}

// NodeScore 节点分数
type NodeScore struct {
	Name  string
	Score int64
}

// NodeScoreList 节点分数列表
type NodeScoreList []NodeScore

// StateData 状态数据接口
type StateData interface {
	Clone() StateData
}

// CycleState 调度周期状态
type CycleState struct {
	data map[string]StateData
}

// NewCycleState 创建周期状态
func NewCycleState() *CycleState {
	return &CycleState{
		data: make(map[string]StateData),
	}
}

// Write 写入状态
func (c *CycleState) Write(key string, val StateData) {
	c.data[key] = val
}

// Read 读取状态
func (c *CycleState) Read(key string) (StateData, error) {
	if v, ok := c.data[key]; ok {
		return v, nil
	}
	return nil, &NotFoundError{Key: key}
}

// NotFoundError 未找到错误
type NotFoundError struct {
	Key string
}

func (e *NotFoundError) Error() string {
	return "state not found: " + e.Key
}

// NodeInfo 节点信息
type NodeInfo struct {
	node *v1.Node
}

// NewNodeInfo 创建节点信息
func NewNodeInfo(node *v1.Node) *NodeInfo {
	return &NodeInfo{node: node}
}

// Node 获取节点
func (n *NodeInfo) Node() *v1.Node {
	return n.node
}

// Plugin 插件基础接口
type Plugin interface {
	Name() string
}

// PreFilterPlugin 预过滤插件接口
type PreFilterPlugin interface {
	Plugin
	PreFilter(ctx context.Context, state *CycleState, pod *v1.Pod) *Status
}

// FilterPlugin 过滤插件接口
type FilterPlugin interface {
	Plugin
	Filter(ctx context.Context, state *CycleState, pod *v1.Pod, nodeInfo *NodeInfo) *Status
}

// ScorePlugin 打分插件接口
type ScorePlugin interface {
	Plugin
	Score(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) (int64, *Status)
}

// ScoreExtensions 打分扩展接口
type ScoreExtensions interface {
	NormalizeScore(ctx context.Context, state *CycleState, pod *v1.Pod, scores NodeScoreList) *Status
}

// ReservePlugin 预留插件接口
type ReservePlugin interface {
	Plugin
	Reserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	Unreserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)
}
