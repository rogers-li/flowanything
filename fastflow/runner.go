package fastflow

import (
	"context"
	"errors"
	"flow-anything/eventbus"
	"fmt"
	"time"
)

type Runner struct {
	bus                 *eventbus.EventBus
	nodeExecutor        *NodeExecutor
	flowInstanceManager *FlowInstanceManager
}

func NewRunner(bus *eventbus.EventBus) *Runner {
	flowInstanceManager := NewFlowInstanceManager()
	nodeExecutor := NewNodeExecutor()
	runner := &Runner{
		bus:                 bus,
		flowInstanceManager: flowInstanceManager,
		nodeExecutor:        nodeExecutor,
	}
	flowEventHandler := NewFlowEventSubscriber(flowInstanceManager, nodeExecutor, runner.callback)
	bus.Register(flowEventHandler)
	return runner
}

func (r *Runner) AddNodeProcessor(nodeType string, nodeProcessor INodeProcessor) *Runner {
	r.nodeExecutor.AddNodeProcessor(nodeType, nodeProcessor)
	return r
}

func (r *Runner) Run(flow *Flow, inputCtx map[string]interface{}) (map[string]interface{}, error) {
	flowInstance, future := r.flowInstanceManager.CreateInstance(flow, r.bus, r.nodeExecutor)
	// 开始启动执行流程
	go func() {
		flowEventData := CreateFlowEventData(nil, StartFlowEventData{input: inputCtx}, flowInstance.instanceId)
		event := eventbus.CreateEvent(EventTypeStartFlow, flowEventData)
		r.bus.Post(event)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*200)

	defer cancel()
	// 同步等待流程执行结果
	select {
	case result := <-future:
		// time.Sleep(time.Second * 20)
		fmt.Println("========== get result =========")
		if !result.isSuccess {
			return nil, errors.New(result.failMsg)
		}
		return flowInstance.flowCtx.output, nil
	case <-ctx.Done():
		return nil, errors.New("timeout for waiting on response")
	}
}

func (r *Runner) callback(instance *FlowInstance, result *FlowResult) {
	c, err := r.flowInstanceManager.GetInstanceChan(instance.instanceId)
	if err == nil {
		c <- result
	}
}
