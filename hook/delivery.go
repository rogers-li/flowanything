package hook

import "flow-anything/eventbus"

type EventDeliveryByChannel struct {
	c chan eventbus.Event
	EventDelivery
}

func NewEventDeliveryByChannel(c chan eventbus.Event) *EventDeliveryByChannel {
	return &EventDeliveryByChannel{c: c}
}

func (e *EventDeliveryByChannel) Deliver(event eventbus.Event) {
	e.c <- event
}
