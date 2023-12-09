package eventbus

const (
	systemEventTypeDeadEvent = "DeadEvent"
)

type Event struct {
	// 事件类型
	EventType string
	// 事件数据
	EventData interface{}
	// 事件流水记录，用于扩展
	// 如果时间是有依赖关系的，并且处理当前事件依赖于前置的事件，则可以将前置时间放到pipeline中
	EventPipeline []Event
	// 发送事件的event bus
	FromBus *EventBus
}

func CreateEvent(eventType string, eventData interface{}) Event {
	event := Event{EventType: eventType, EventData: eventData}
	return event
}
