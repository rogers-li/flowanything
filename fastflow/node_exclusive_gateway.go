package fastflow

import (
	"flow-anything/calculate"
	"flow-anything/eventbus"
	"flow-anything/utils"
)

// ExclusiveNodeProcessor 排他网关的实现
// 由于排他网关是一个流程编排的基础能力，因此将ExclusiveNodeProcessor方案flow.go文件种
type ExclusiveNodeProcessor struct {
	CommonProcessor
	calculate.Expression
}

func (e *ExclusiveNodeProcessor) BeforeExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	return true, nil
}

func (e *ExclusiveNodeProcessor) Execute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) (bool, error) {
	var nodeData ExclusiveNodeData
	err := utils.ReConstruct(node.NodeData, &nodeData)
	if err != nil {
		return false, err
	}
	for _, condition := range nodeData.Conditions {
		result, _ := e.Expression.ValByBool(condition.Expression, flowCtx.runtimeCtx)
		if result {
			for _, assign := range condition.Assign {
				if len(assign.AssignExpression) > 0 {
					_ = e.Expression.ValAssign(assign.AssignExpression, flowCtx.runtimeCtx)
					flowEventData := CreateFlowEventData(node, NextNodeEventData{NextNodeId: condition.Downstream.ToNodeId}, flowCtx.flowInstanceId)
					event := eventbus.CreateEvent(EventTypeNextNode, flowEventData)
					bus.Post(event)
					return true, nil
				}
			}
		}
	}
	for _, assign := range nodeData.DefaultCondition.Assign {
		if len(assign.AssignExpression) > 0 {
			_ = e.Expression.ValAssign(assign.AssignExpression, flowCtx.runtimeCtx)
		}
	}
	flowEventData := CreateFlowEventData(node, NextNodeEventData{NextNodeId: nodeData.DefaultCondition.Downstream.ToNodeId}, flowCtx.flowInstanceId)
	event := eventbus.CreateEvent(EventTypeNextNode, flowEventData)
	bus.Post(event)
	return true, nil
}

func (e *ExclusiveNodeProcessor) AfterExecute(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus) error {
	return e.CommonProcessor.AfterExecute(flowCtx, node, bus)
}

func (e *ExclusiveNodeProcessor) ExecuteFailed(flowCtx *FlowCtx, node *Node, bus *eventbus.EventBus, err error) {
	e.CommonProcessor.ExecuteFailed(flowCtx, node, bus, err)
}
