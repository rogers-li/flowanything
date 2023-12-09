package calculate_node

import (
	"flow-anything/eventbus"
	"flow-anything/fastflow"
)

type CalculateNodeProcessor struct {
	fastflow.CommonProcessor
}

func NewCalculateNodeProcessor() *CalculateNodeProcessor {
	return &CalculateNodeProcessor{}
}

func (c *CalculateNodeProcessor) BeforeExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (c *CalculateNodeProcessor) Execute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {

	return true, nil
}

func (c *CalculateNodeProcessor) AfterExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) error {
	return c.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (c *CalculateNodeProcessor) ExecuteFailed(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus, err error) {
	c.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}
