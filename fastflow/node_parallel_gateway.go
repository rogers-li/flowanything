package fastflow

import (
	"flow-anything/eventbus"
	"flow-anything/utils"
)

// ParallelNodeProcessor 并发网关的实现
// 由于并发网关是一个流程编排的基础能力，因此将ParallelNodeProcessor方案flow.go文件中
type ParallelNodeProcessor struct {
	CommonProcessor
}

func (p *ParallelNodeProcessor) BeforeExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	var nodeData ParallelNodeData
	err := utils.ReConstruct(node.NodeData, &nodeData)
	if err != nil {
		return false, err
	}
	upstreams := nodeData.Upstream
	for _, stream := range upstreams {
		if !flowCtx.IsNodeFinished(stream.FromNodeId) {
			return false, nil
		}
	}
	return true, nil
}

func (p *ParallelNodeProcessor) Execute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	var nodeData ParallelNodeData
	err := utils.ReConstruct(node.NodeData, &nodeData)
	if err != nil {
		return false, err
	}
	downStreams := nodeData.Downstream
	for _, stream := range downStreams {
		toNodeId := stream.ToNodeId
		go func() {
			flowEventData := CreateFlowEventData(node, NextNodeEventData{NextNodeId: toNodeId}, flowCtx.flowInstanceId)
			event := eventbus.CreateEvent(EventTypeNextNode, flowEventData)
			bus.Post(event)
		}()
	}
	return true, nil
}

func (p *ParallelNodeProcessor) AfterExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) error {
	return p.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (p *ParallelNodeProcessor) ExecuteFailed(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus, err error) {
	p.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}
