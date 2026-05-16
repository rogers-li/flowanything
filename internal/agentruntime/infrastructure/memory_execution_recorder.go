package infrastructure

import (
	"context"
	"sync"

	"flow-anything/internal/platform/contracts/tool"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type MemoryExecutionRecorder struct {
	mu      sync.RWMutex
	records map[string]tool.ExecutionRecord
}

func NewMemoryExecutionRecorder() *MemoryExecutionRecorder {
	return &MemoryExecutionRecorder{
		records: make(map[string]tool.ExecutionRecord),
	}
}

func (r *MemoryExecutionRecorder) RecordStarted(ctx context.Context, record tool.ExecutionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records[record.CallID.String()] = record
	return nil
}

func (r *MemoryExecutionRecorder) RecordFinished(ctx context.Context, record tool.ExecutionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.records[record.CallID.String()] = record
	return nil
}

func (r *MemoryExecutionRecorder) GetExecution(ctx context.Context, tenantID tenant.ID, callID id.ID) (tool.ExecutionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[callID.String()]
	if !ok || record.TenantID != tenantID {
		return tool.ExecutionRecord{}, apperrors.New(apperrors.CodeNotFound, "tool execution not found")
	}
	return record, nil
}

func (r *MemoryExecutionRecorder) Get(callID string) (tool.ExecutionRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.records[callID]
	return record, ok
}
