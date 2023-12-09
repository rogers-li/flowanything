package fastflow

import "sync"

const (
	nodePending = iota
	nodeProcessing
	nodeCompleted
)

type InputCtx map[string]interface{}
type RuntimeCtx map[string]interface{}
type OutputCtx map[string]interface{}
type NodeCtx struct {
	startTime int64
	endTime   int64
	moreData  interface{}
}

type FlowCtx struct {
	flowInstanceId string
	input          InputCtx
	runtimeCtx     RuntimeCtx          // 运行上下文，用于流程执行时的条件判断等，key为变量ID
	output         OutputCtx           // 运行结果，用于存放流程结果数据，流程结束后返回给调用方，具体数据结构由上层应用实现
	nodesCtx       map[string]*NodeCtx // 节点运行上下文，对于有状态的节点，将状态信息存到此字段上
	nodeStatus     map[string]int      // 存放节点的执行状态，key为节点编号
	lock           sync.Locker
}

func NewFlowCtx(instanceId string) *FlowCtx {
	return &FlowCtx{
		flowInstanceId: instanceId,
		runtimeCtx:     RuntimeCtx{},
		nodesCtx:       map[string]*NodeCtx{},
		output:         OutputCtx{},
		nodeStatus:     map[string]int{},
	}
}

func (f *FlowCtx) SetNodeContext(nodeId string, nodeCtx *NodeCtx) {
	// todo add lock
	f.nodesCtx[nodeId] = nodeCtx
}

func (f *FlowCtx) RecordFinishedNode(nodeId string) {
	f.nodeStatus[nodeId] = nodeCompleted
}

func (f *FlowCtx) IsNodeFinished(nodeId string) bool {
	status, ok := f.nodeStatus[nodeId]
	if ok && status == nodeCompleted {
		return true
	}
	return false
}

func (f *FlowCtx) GetNodeContext(nodeId string) *NodeCtx {
	// todo add lock
	nodeCtx, ok := f.nodesCtx[nodeId]
	if !ok {
		return nil
	} else {
		return nodeCtx
	}
}

func (f *FlowCtx) GetFlowRuntimeCtx() RuntimeCtx {
	return f.runtimeCtx
}

type InitResultCtx func() interface{}

// GetAndInitResultCtx 由于可能会并发初始化结果数据，因此需要将get和init封装成原子操作
// 应用节点提供initFunc，初始化自己的数据结构
func (f *FlowCtx) GetAndInitResultCtx(key string, initFunc InitResultCtx) interface{} {
	f.lock.Lock()
	defer func() {
		f.lock.Unlock()
	}()
	resultByKey, ok := f.output[key]
	if !ok || resultByKey == nil {
		resultByKey = initFunc()
		f.output[key] = resultByKey
	}
	return resultByKey
}
