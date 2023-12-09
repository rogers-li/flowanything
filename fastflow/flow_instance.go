package fastflow

import (
	"flow-anything/eventbus"
	"fmt"

	"github.com/google/uuid"
)

const (
	statusCompleted = "Completed"
	statusFailed    = "Failed"
)

type FlowInstance struct {
	instanceId     string
	instanceStatus string
	flow           *Flow
	flowCtx        *FlowCtx
	bus            *eventbus.EventBus
	nodeExecutor   *NodeExecutor
}

func CreateInstance(flow *Flow, bus *eventbus.EventBus, nodeExecutor *NodeExecutor) *FlowInstance {
	instanceId := uuid.New().String()
	flowCtx := NewFlowCtx(instanceId)
	return &FlowInstance{
		instanceId:   instanceId,
		flow:         flow,
		flowCtx:      flowCtx,
		bus:          bus,
		nodeExecutor: nodeExecutor,
	}
}

func (f *FlowInstance) StartFlow() {
	firstNodeId := f.flow.RootNode
	f.executeNode(firstNodeId)
}

func (f *FlowInstance) EndFlow() {
	// todo
}

func (f *FlowInstance) NodeCompletedCallBack(nodeId string) {
	node, err := f.getNode(nodeId)
	if err != nil {
		// todo
	} else {
		f.flowCtx.RecordFinishedNode(nodeId)
		nextNodeId := node.Downstream.ToNodeId
		if nextNodeId == "" {
			// 流程结束
			flowEventData := CreateFlowEventData(node, EndFlowEventData{}, f.flowCtx.flowInstanceId)
			event := eventbus.CreateEvent(EventTypeEndFlow, flowEventData)
			f.bus.Post(event)
		} else {
			f.executeNode(nextNodeId)
		}
	}
}

func (f *FlowInstance) NextNode(fromNodeId string, nextNodeId string) {
	f.flowCtx.RecordFinishedNode(fromNodeId)
	node, _ := f.getNode(fromNodeId)
	if nextNodeId == "" {
		// 流程结束
		flowEventData := CreateFlowEventData(node, EndFlowEventData{}, f.flowCtx.flowInstanceId)
		event := eventbus.CreateEvent(EventTypeEndFlow, flowEventData)
		f.bus.Post(event)
	} else {
		f.executeNode(nextNodeId)
	}
}

func (f *FlowInstance) getNode(nodeId string) (*Node, error) {
	node, ok := f.flow.Nodes[nodeId]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeId)
	}
	return node, nil
}

func (f *FlowInstance) executeNode(nodeId string) error {
	node, err := f.getNode(nodeId)
	if err != nil {
		return err
	}
	return f.nodeExecutor.ExecuteNode(f.flowCtx, node, f.bus)
}
