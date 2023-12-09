package eventbus

type EventBus struct {
	dispatcher  Dispatcher
	subscribers SubscriberRegistry
}

func NewEventBus(dispatcher Dispatcher, subscribers SubscriberRegistry) *EventBus {
	bus := &EventBus{
		dispatcher:  dispatcher,
		subscribers: subscribers,
	}
	return bus
}

func NewDefaultEventBus() *EventBus {
	factory := DispatcherFactory{}
	dispatcher := factory.Concurrent()
	subscribers := NewSubscriberRegistry()
	return NewEventBus(dispatcher, subscribers)
}

// Register 定义者注册
// 具体注册逻辑由SubscriberRegistry实现
func (eb *EventBus) Register(subscriber Subscriber) {
	eb.subscribers.Register(eb, subscriber)
}

func (eb *EventBus) UnRegister(subscriber interface{}) {
	eb.subscribers.UnRegister(subscriber)
}

// Post 向eventbus发送一个event
func (eb *EventBus) Post(event Event) {
	event.FromBus = eb
	subscriptions := eb.subscribers.GetAllSubscriptions(event)
	if len(subscriptions) > 0 {
		eb.dispatcher.Dispatch(event, subscriptions)
	} else if event.EventType != systemEventTypeDeadEvent {
		// 如果event没有订阅者，则重新封装一个DeadEvent发送到eventbus
		// 上层应用可以注册一个监听DeadEvent的订阅者兜底
		deadEvent := Event{
			EventType:     systemEventTypeDeadEvent,
			EventData:     event.EventData,
			EventPipeline: []Event{event},
		}
		eb.Post(deadEvent)
	}
}
