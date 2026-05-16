package connector

import "fmt"

// Repository resolves connectors and operations. Product implementations can
// back this with database snapshots, config service, or in-memory fixtures.
type Repository interface {
	GetConnector(id string) (ConnectorSpec, bool)
	GetOperation(id string) (OperationSpec, bool)
}

type MemoryRepository struct {
	connectors map[string]ConnectorSpec
	operations map[string]OperationSpec
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		connectors: map[string]ConnectorSpec{},
		operations: map[string]OperationSpec{},
	}
}

func (r *MemoryRepository) RegisterConnector(connector ConnectorSpec) error {
	if connector.ID == "" {
		return fmt.Errorf("connector id is required")
	}
	r.connectors[connector.ID] = connector
	return nil
}

func (r *MemoryRepository) RegisterOperation(operation OperationSpec) error {
	if operation.ID == "" {
		return fmt.Errorf("operation id is required")
	}
	if operation.ConnectorID == "" {
		return fmt.Errorf("operation connector_id is required")
	}
	r.operations[operation.ID] = operation
	return nil
}

func (r *MemoryRepository) GetConnector(id string) (ConnectorSpec, bool) {
	connector, ok := r.connectors[id]
	return connector, ok
}

func (r *MemoryRepository) GetOperation(id string) (OperationSpec, bool) {
	operation, ok := r.operations[id]
	return operation, ok
}
