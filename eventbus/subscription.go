package eventbus

type Handler func(event Event)

// Subscription 事件订阅的封装
type Subscription struct {
	EventType string
	// 订阅者的处理方法
	handler Handler
}

func NewSubscription(eventType string, handler Handler) *Subscription {
	s := &Subscription{
		EventType: eventType,
		handler:   handler,
	}
	return s
}

// DispatchEvent 执行订阅者的处理函数
func (s *Subscription) DispatchEvent(event Event) {
	s.handler(event)
}
