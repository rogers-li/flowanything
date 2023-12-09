package collect_node

import (
	"flow-anything/eventbus"
	"flow-anything/fastflow"
	"flow-anything/utils"
)

type CollectNodeProcessor struct {
	fastflow.CommonProcessor
}

func NewCollectNodeProcessor() *CollectNodeProcessor {
	return &CollectNodeProcessor{}
}

func (c *CollectNodeProcessor) BeforeExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (c *CollectNodeProcessor) Execute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) (bool, error) {

	return true, nil
}

func (c *CollectNodeProcessor) AfterExecute(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus) error {
	return c.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (c *CollectNodeProcessor) ExecuteFailed(flowCtx *fastflow.FlowCtx, node *fastflow.Node, bus *eventbus.EventBus, err error) {
	c.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}

func (c *CollectNodeProcessor) collectFields(flowCtx *fastflow.FlowCtx, nodeData CollectNodeData) error {
	if len(nodeData.CollectFields) <= 0 {
		return nil
	}
	resultCtx := flowCtx.GetAndInitResultCtx(collectResultCtxKey, c.newResult)
	apiResult := resultCtx.(Result)
	runtimeCtx := flowCtx.GetFlowRuntimeCtx()
	for _, field := range nodeData.CollectFields {
		name := field.Target.FieldName
		value, err := utils.GetValByPath(runtimeCtx, field.Source.FieldExpression)
		if err != nil {
			return err
		}
		apiResult.lock.Lock()
		apiResult.Variables[name] = value
		apiResult.lock.Unlock()
	}
	return nil
}

func (c *CollectNodeProcessor) newResult() interface{} {
	return Result{}
}
