package hook

import (
	"flow-anything/eventbus"
	"flow-anything/fastflow"
	"fmt"
)

type FlowTracer struct {
	eventChannel chan eventbus.Event
}

func NewFlowTracer(bus *eventbus.EventBus) *FlowTracer {
	eventChannel := make(chan eventbus.Event)
	delivery := NewEventDeliveryByChannel(eventChannel)
	h := NewDefaultEventHook(delivery)
	h.Watch(fastflow.EventTypeStartFlow)
	h.Watch(fastflow.EventTypeNextNode)
	h.Watch(fastflow.EventTypeFlowFailed)
	h.Watch(fastflow.EventTypeNodeCompleted)
	h.Watch(fastflow.EventTypeNodeFailed)
	h.BindEventBus(bus)
	return &FlowTracer{eventChannel: eventChannel}
}

func (f *FlowTracer) StartTracing() {
	go func() {
		for {
			select {
			case e := <-f.eventChannel:
				f.printEvent(e)
			}
		}
	}()
}

func (f *FlowTracer) printEvent(e eventbus.Event) {
	if e.EventType == fastflow.EventTypeNodeCompleted {
		eventData := e.EventData.(fastflow.FlowEventData)
		data := eventData.EventData.(fastflow.NodeCompleteEventData)
		fmt.Printf("trace: event[%s] node_id[%s]\n", e.EventType, data.NodeId)
	} else if e.EventType == fastflow.EventTypeNextNode {
		eventData := e.EventData.(fastflow.FlowEventData)
		data := eventData.EventData.(fastflow.NextNodeEventData)
		fmt.Printf("trace: event[%s] from_node_id[%s] to_node_id[%s]\n", e.EventType, data.FromNodeId, data.NextNodeId)
	} else {
		fmt.Printf("trace: %s\n", e.EventType)
	}
}
