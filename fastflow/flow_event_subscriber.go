package fastflow

import (
	"flow-anything/eventbus"
)

type callback func(instance *FlowInstance, result *FlowResult)

type FlowResult struct {
	isSuccess bool
	failMsg   string
	result    OutputCtx
}

type FlowEventSubscriber struct {
	flowInstanceManager  *FlowInstanceManager
	nodeExecutor         *NodeExecutor
	callbackWhenFlowStop callback
	eventbus.Subscriber
}

func NewFlowEventSubscriber(flowInstanceManager *FlowInstanceManager, nodeExecutor *NodeExecutor, flowStopCallback callback) *FlowEventSubscriber {
	return &FlowEventSubscriber{
		flowInstanceManager:  flowInstanceManager,
		nodeExecutor:         nodeExecutor,
		callbackWhenFlowStop: flowStopCallback,
	}
}

func (f *FlowEventSubscriber) Subscript() []*eventbus.Subscription {
	subscriptions := make([]*eventbus.Subscription, 0)
	subscriptions = append(subscriptions, eventbus.NewSubscription(EventTypeStartFlow, f.handleEventStartFlow))
	subscriptions = append(subscriptions, eventbus.NewSubscription(EventTypeNextNode, f.handleEventNextNode))
	subscriptions = append(subscriptions, eventbus.NewSubscription(EventTypeNodeCompleted, f.handleEventNodeCompleted))
	subscriptions = append(subscriptions, eventbus.NewSubscription(EventTypeEndFlow, f.handleEventEndFlow))
	subscriptions = append(subscriptions, eventbus.NewSubscription(EventTypeFlowFailed, f.handleEventFlowFailed))
	return subscriptions
}

// HandleEventStartFlow 处理启动流程事件
// 创建流程实例，流程运行时的上下文数据都保存在流程实例里，并且流程实例会放在FlowEventData数据结构中，随着event一直流转
func (f *FlowEventSubscriber) handleEventStartFlow(event eventbus.Event) {
	flowInstance, err := f.getFlowInstance(event)
	if err == nil {
		flowInstance.StartFlow()
	}
}

// HandleEventNextNode 处理NextNode事件
// 节点跳转是流程编排的能力，因此建议自定义的节点不要发布NextNode事件
// 自定义节点完成后可以发布NodeComplete事件，让流程订阅者来处理
func (f *FlowEventSubscriber) handleEventNextNode(event eventbus.Event) {
	flowInstance, err := f.getFlowInstance(event)
	if err == nil {
		flowEventData := event.EventData.(FlowEventData)
		nextNodeEventData := flowEventData.EventData.(NextNodeEventData)
		flowInstance.NextNode(nextNodeEventData.FromNodeId, nextNodeEventData.NextNodeId)
	}
}

// HandleEventNodeCompleted 处理节点执行完成事件
// 节点完成了直接发布完成事件就好，不需要关心节点的跳转
func (f *FlowEventSubscriber) handleEventNodeCompleted(event eventbus.Event) {
	flowInstance, err := f.getFlowInstance(event)
	if err == nil {
		flowEventData := event.EventData.(FlowEventData)
		nodeCompleteEventData := flowEventData.EventData.(NodeCompleteEventData)
		flowInstance.NodeCompletedCallBack(nodeCompleteEventData.NodeId)
	}
}

// HandleEventEndFlow todo 目前貌似没有场景需要流程结束后执行，这个事件待扩展
func (f *FlowEventSubscriber) handleEventEndFlow(event eventbus.Event) {
	flowInstance, err := f.getFlowInstance(event)
	if err == nil {
		flowInstance.EndFlow()
	}
	f.callbackWhenFlowStop(flowInstance, &FlowResult{
		isSuccess: false,
		failMsg:   "",
		result:    flowInstance.flowCtx.output,
	})
}

// HandleEventFlowFailed todo 目前貌似没有场景需要流程结束后执行，这个事件待扩展
func (f *FlowEventSubscriber) handleEventFlowFailed(event eventbus.Event) {
	flowInstance, err := f.getFlowInstance(event)
	flowEventData := event.EventData.(FlowEventData)
	nodeFailedEventData := flowEventData.EventData.(NodeFailedEventData)
	if err == nil {
		f.callbackWhenFlowStop(flowInstance, &FlowResult{
			isSuccess: true,
			failMsg:   nodeFailedEventData.failedMsg,
			result:    flowInstance.flowCtx.output,
		})
	}
}

func (f *FlowEventSubscriber) getFlowInstance(event eventbus.Event) (*FlowInstance, error) {
	eventData := event.EventData
	flowEventData := eventData.(FlowEventData)
	return f.flowInstanceManager.GetInstance(flowEventData.flowInstanceId)
}
