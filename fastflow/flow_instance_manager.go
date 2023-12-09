package fastflow

import (
	"flow-anything/eventbus"
	"fmt"
)

type FlowInstanceManager struct {
	instancePool       map[string]*FlowInstance
	instanceResultChan map[string]chan *FlowResult
}

func NewFlowInstanceManager() *FlowInstanceManager {
	return &FlowInstanceManager{
		instancePool:       map[string]*FlowInstance{},
		instanceResultChan: map[string]chan *FlowResult{},
	}
}

func (f *FlowInstanceManager) CreateInstance(flow *Flow, useBus *eventbus.EventBus, executor *NodeExecutor) (*FlowInstance, chan *FlowResult) {
	flowInstance := CreateInstance(flow, useBus, executor)
	f.instancePool[flowInstance.instanceId] = flowInstance
	c := make(chan *FlowResult)
	f.instanceResultChan[flowInstance.instanceId] = c
	return flowInstance, c
}

func (f *FlowInstanceManager) GetInstance(instanceId string) (*FlowInstance, error) {
	instance, ok := f.instancePool[instanceId]
	if !ok {
		return nil, fmt.Errorf("instance not found[%s]", instanceId)
	}
	return instance, nil
}

func (f *FlowInstanceManager) GetInstanceChan(instanceId string) (chan *FlowResult, error) {
	c, ok := f.instanceResultChan[instanceId]
	if !ok {
		return nil, fmt.Errorf("instance not found[%s]", instanceId)
	}
	return c, nil
}
