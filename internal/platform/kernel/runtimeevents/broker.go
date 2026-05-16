package runtimeevents

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/runtimeevent"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

const defaultHistoryLimit = 256

type Broker struct {
	historyLimit int
	mu           sync.RWMutex
	history      map[string][]runtimeevent.Event
	subscribers  map[string]map[chan runtimeevent.Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		historyLimit: defaultHistoryLimit,
		history:      map[string][]runtimeevent.Event{},
		subscribers:  map[string]map[chan runtimeevent.Event]struct{}{},
	}
}

func (b *Broker) Publish(ctx context.Context, event runtimeevent.Event) error {
	if event.TraceID == "" {
		return nil
	}
	if event.ID == "" {
		event.ID = id.New("rtevt").String()
	}
	key := eventKey(event.TenantID, event.TraceID)

	b.mu.Lock()
	history := append(b.history[key], event)
	if len(history) > b.historyLimit {
		history = append([]runtimeevent.Event(nil), history[len(history)-b.historyLimit:]...)
	}
	b.history[key] = history

	subscribers := make([]chan runtimeevent.Event, 0, len(b.subscribers[key]))
	for ch := range b.subscribers[key] {
		subscribers = append(subscribers, ch)
	}
	b.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (b *Broker) Subscribe(ctx context.Context, tenantID tenant.ID, traceID string) (<-chan runtimeevent.Event, func()) {
	ch := make(chan runtimeevent.Event, 64)
	key := eventKey(tenantID, traceID)

	b.mu.Lock()
	if _, ok := b.subscribers[key]; !ok {
		b.subscribers[key] = map[chan runtimeevent.Event]struct{}{}
	}
	b.subscribers[key][ch] = struct{}{}
	history := append([]runtimeevent.Event(nil), b.history[key]...)
	b.mu.Unlock()

	go func() {
		for _, event := range history {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
		<-ctx.Done()
	}()

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subscribers[key], ch)
		if len(b.subscribers[key]) == 0 {
			delete(b.subscribers, key)
		}
	}
	return ch, cancel
}

func eventKey(tenantID tenant.ID, traceID string) string {
	return tenantID.String() + "/" + traceID
}
