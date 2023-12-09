package eventbus

const registryMethodNamePrefix = "HandleEvent"

// SubscriberRegistry 订阅者注册
type SubscriberRegistry struct {
	subscriptions map[string][]*Subscription
}

func NewSubscriberRegistry() SubscriberRegistry {
	registry := SubscriberRegistry{
		subscriptions: map[string][]*Subscription{},
	}
	return registry
}

func (s *SubscriberRegistry) Register(bus *EventBus, subscriber Subscriber) {
	for _, subscription := range subscriber.Subscript() {
		eventType := subscription.EventType
		eventSubscribers, ok := s.subscriptions[eventType]
		// todo 加锁
		if !ok {
			eventSubscribers = make([]*Subscription, 0)
		}
		eventSubscribers = append(eventSubscribers, subscription)
		s.subscriptions[eventType] = eventSubscribers
	}
}

func (s *SubscriberRegistry) UnRegister(subscriber interface{}) {

}

// GetAllSubscriptions 获取所有的订阅了此event的订阅
func (s *SubscriberRegistry) GetAllSubscriptions(event Event) []*Subscription {
	eventSubscriptions, ok := s.subscriptions[event.EventType]
	if !ok {
		return make([]*Subscription, 0, 0)
	} else {
		return eventSubscriptions
	}

}
