package eventbus

// Dispatcher eventbus只是封装易用的接口，事件的调度由Dispatcher实现
// 上层应用可以实现自己的Dispatcher，例如基于异步缓存队列的Dispatcher
type Dispatcher interface {
	Dispatch(event Event, subscribers []*Subscription)
}

type DispatcherFactory struct {
}

func (d *DispatcherFactory) Serial() Dispatcher {
	dispatcher := serialDispatcher{}
	return dispatcher
}

func (d *DispatcherFactory) Concurrent() Dispatcher {
	dispatcher := concurrentDispatcher{}
	return dispatcher
}

// 串行执行的Dispatcher
// 事件的发布和事件的执行都使用同一个协程
// 注意：如果一个event在处理过程中，产生另外的event，会执行完新产生的event后再执行原来的event
// 例如，e1 -> e2 -> e3,事件的处理完成顺序为 e3,e2,e1
type serialDispatcher struct {
	Dispatcher
}

func (s serialDispatcher) Dispatch(event Event, subscriptions []*Subscription) {
	for _, subscription := range subscriptions {
		subscription.DispatchEvent(event)
	}
}

// 并发执行的Dispatcher
// 所有事件，所有订阅这都使用新的协程执行，使用这个Dispatcher需要考虑事件是否存在依赖关系
// 事件并发大的时候，会使用大量协程
type concurrentDispatcher struct {
	Dispatcher
}

func (c concurrentDispatcher) Dispatch(event Event, subscriptions []*Subscription) {
	for _, subscription := range subscriptions {
		go subscription.DispatchEvent(event)
	}
}
