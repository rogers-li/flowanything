package hook

import "flow-anything/eventbus"

type EventWatcher interface {
	Watch(eventType string)
	GetWatchList() []string
}

type DefaultEventWatcher struct {
	EventWatcher
	subscriptEventSet map[string]bool
}

func (w *DefaultEventWatcher) Watch(eventType string) {
	w.subscriptEventSet[eventType] = true
}

func (w *DefaultEventWatcher) GetWatchList() []string {
	watchList := make([]string, 0)
	for k, _ := range w.subscriptEventSet {
		watchList = append(watchList, k)
	}
	return watchList
}

type EventDelivery interface {
	Deliver(event eventbus.Event)
}

type Hook struct {
	EventWatcher
	EventDelivery
	eventbus.Subscriber
}

func NewEventHook(watcher EventWatcher, delivery EventDelivery) *Hook {
	return &Hook{
		EventWatcher:  watcher,
		EventDelivery: delivery,
	}
}

func NewDefaultEventHook(delivery EventDelivery) *Hook {
	return NewEventHook(&DefaultEventWatcher{subscriptEventSet: map[string]bool{}}, delivery)
}

func (h *Hook) Subscript() []*eventbus.Subscription {
	subscriptions := make([]*eventbus.Subscription, 0)
	for _, eventType := range h.GetWatchList() {
		subscriptions = append(subscriptions, eventbus.NewSubscription(eventType, h.Deliver))
	}
	return subscriptions
}

func (h *Hook) BindEventBus(bus *eventbus.EventBus) {
	bus.Register(h)
}
