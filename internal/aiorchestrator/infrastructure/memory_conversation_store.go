package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/aiorchestrator/domain"
	"flow-anything/internal/platform/contracts/model"
)

type MemoryConversationStore struct {
	mu       sync.RWMutex
	messages map[string][]model.Message
}

func NewMemoryConversationStore() *MemoryConversationStore {
	return &MemoryConversationStore{
		messages: make(map[string][]model.Message),
	}
}

// LoadMessages returns the most recent non-system messages for one conversation.
// The caller owns the returned slice, so later store writes cannot mutate the
// message history already assembled for an LLM request.
func (s *MemoryConversationStore) LoadMessages(ctx context.Context, ref domain.ConversationRef, limit int) ([]model.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := s.messages[ref.Key()]
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}

	result := make([]model.Message, len(items))
	copy(result, items)
	return result, nil
}

// AppendMessages adds one completed turn to the in-memory conversation history.
// System messages are intentionally not persisted because they are derived from
// the latest Agent configuration on every turn.
func (s *MemoryConversationStore) AppendMessages(ctx context.Context, ref domain.ConversationRef, messages []model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, message := range messages {
		if message.Role == model.RoleSystem {
			continue
		}
		s.messages[ref.Key()] = append(s.messages[ref.Key()], message)
	}
	return nil
}
