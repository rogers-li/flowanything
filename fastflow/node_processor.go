package fastflow

import (
	"flow-anything/eventbus"
	"fmt"
)

type INodeProcessor interface {
	BeforeExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error)
	Execute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error)
	AfterExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) error
	ExecuteFailed(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus, err error)
}

type NodeProcessorFactory struct {
	nodeProcessors         map[string]INodeProcessor
	exclusiveNodeProcessor *ExclusiveNodeProcessor
	parallelNodeProcessor  *ParallelNodeProcessor
}

func NewNodeProcessorFactory() *NodeProcessorFactory {
	n := &NodeProcessorFactory{
		nodeProcessors: map[string]INodeProcessor{},
	}
	n.RegistryNode(nodeTypeExclusive, &ExclusiveNodeProcessor{})
	n.RegistryNode(nodeTypeParallel, &ParallelNodeProcessor{})
	return n
}

func (nf *NodeProcessorFactory) RegistryNode(nodeType string, nodeProcessor INodeProcessor) {
	nf.nodeProcessors[nodeType] = nodeProcessor
}

func (nf *NodeProcessorFactory) GetNodeProcessor(nodeType string) (INodeProcessor, error) {
	node, ok := nf.nodeProcessors[nodeType]
	if !ok {
		return nil, fmt.Errorf("node implement not found")
	} else {
		return node, nil
	}
}

type CommonProcessor struct {
	INodeProcessor
}

func (c *CommonProcessor) BeforeExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (c *CommonProcessor) Execute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (c *CommonProcessor) AfterExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) error {
	flowEventData := CreateFlowEventData(node, NodeCompleteEventData{NodeId: node.NodeID}, flowCtx.flowInstanceId)
	event := eventbus.CreateEvent(EventTypeNodeCompleted, flowEventData)
	bus.Post(event)
	return nil
}

func (c *CommonProcessor) ExecuteFailed(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus, err error) {
	flowEventData := CreateFlowEventData(node, NodeFailedEventData{nodeId: node.NodeID, failedMsg: err.Error()}, flowCtx.flowInstanceId)
	event := eventbus.CreateEvent(EventTypeNodeFailed, flowEventData)
	bus.Post(event)
}
