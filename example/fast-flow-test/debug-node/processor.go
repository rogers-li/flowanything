package fast_flow_test

import (
	"flow-anything/eventbus"
	"flow-anything/fastflow"
	"flow-anything/utils"
	"fmt"
)

type DebugNodeProcessor struct {
	fastflow.CommonProcessor
}

func NewDebugNodeProcessor() *DebugNodeProcessor {
	return &DebugNodeProcessor{}
}

func (d *DebugNodeProcessor) BeforeExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (d *DebugNodeProcessor) Execute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	var nodeData DebugNodeData
	err := utils.ReConstruct(node.NodeData, &nodeData)
	if err != nil {
		return false, err
	}
	fmt.Println(nodeData.PrintText)
	return true, nil
}

func (d *DebugNodeProcessor) AfterExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) error {
	return d.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (d *DebugNodeProcessor) ExecuteFailed(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus, err error) {
	d.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}
