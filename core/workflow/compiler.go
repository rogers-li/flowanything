package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"flow-anything/core/flowengine"
)

// Compiler validates and freezes WorkflowDocument into a runnable snapshot.
//
// It intentionally performs a thin transformation:
// - normalize ids/defaults
// - validate node/edge references and registered node types
// - freeze a deterministic snapshot hash
//
// It does not create hidden nodes or rewrite business semantics.
type Compiler struct {
	registry     *flowengine.Registry
	snapshotIDFn func(document WorkflowDocument, hash string) string
	nowFn        func() time.Time
}

type CompilerOption func(*Compiler)

func WithSnapshotIDFunc(fn func(document WorkflowDocument, hash string) string) CompilerOption {
	return func(c *Compiler) { c.snapshotIDFn = fn }
}

func WithNowFunc(fn func() time.Time) CompilerOption {
	return func(c *Compiler) { c.nowFn = fn }
}

func NewCompiler(registry *flowengine.Registry, opts ...CompilerOption) *Compiler {
	if registry == nil {
		registry = flowengine.NewDefaultRegistry()
	} else {
		_ = flowengine.RegisterControlNodes(registry)
	}
	compiler := &Compiler{
		registry: registry,
		snapshotIDFn: func(document WorkflowDocument, hash string) string {
			return fmt.Sprintf("wf_snapshot_%s", hash[:12])
		},
		nowFn: time.Now,
	}
	for _, opt := range opts {
		opt(compiler)
	}
	return compiler
}

func (c *Compiler) Compile(ctx context.Context, document WorkflowDocument) (CompiledWorkflow, WorkflowDocument, error) {
	normalized := NormalizeDocument(document)
	if err := c.Validate(ctx, normalized); err != nil {
		return CompiledWorkflow{}, normalized, err
	}
	hash, err := HashSpec(normalized.Spec)
	if err != nil {
		return CompiledWorkflow{}, normalized, err
	}
	snapshotID := c.snapshotIDFn(normalized, hash)
	normalized.Publish.Status = PublishValidated
	normalized.Publish.SnapshotID = snapshotID
	normalized.Publish.SnapshotHash = hash
	normalized.Publish.SourceDocument = normalized.ID
	if normalized.Publish.Revision == 0 {
		normalized.Publish.Revision = 1
	}
	return CompiledWorkflow{
		DocumentID: normalized.ID,
		SnapshotID: snapshotID,
		Spec:       normalized.Spec,
		Hash:       hash,
		CompiledAt: c.nowFn(),
	}, normalized, nil
}

func (c *Compiler) Validate(ctx context.Context, document WorkflowDocument) error {
	if document.ID == "" {
		return fmt.Errorf("workflow document id is required")
	}
	if document.Spec.ID == "" {
		return fmt.Errorf("workflow spec id is required")
	}
	seen := map[string]bool{}
	for _, node := range document.Spec.Nodes {
		if node.ID == "" {
			return fmt.Errorf("workflow node id is required")
		}
		if seen[node.ID] {
			return fmt.Errorf("duplicate workflow node id %q", node.ID)
		}
		seen[node.ID] = true
		executor, ok := c.registry.Get(node.Type)
		if !ok {
			return fmt.Errorf("node executor for type %q is not registered", node.Type)
		}
		if err := executor.Validate(ctx, node); err != nil {
			return fmt.Errorf("validate node %q: %w", node.ID, err)
		}
	}
	for _, edge := range document.Spec.Edges {
		if !seen[edge.From] || !seen[edge.To] {
			return fmt.Errorf("edge %q -> %q references unknown node", edge.From, edge.To)
		}
	}
	return nil
}

// NormalizeDocument fills defaults without changing execution semantics.
func NormalizeDocument(document WorkflowDocument) WorkflowDocument {
	if document.ID == "" {
		document.ID = document.Spec.ID
	}
	if document.Spec.ID == "" {
		document.Spec.ID = document.ID
	}
	if document.Spec.Version == "" {
		document.Spec.Version = "v1"
	}
	if document.Publish.Status == "" {
		document.Publish.Status = PublishDraft
	}
	if document.UI.Nodes == nil {
		document.UI.Nodes = map[string]NodeUIMetadata{}
	}
	if document.UI.Edges == nil {
		document.UI.Edges = map[string]EdgeUIMetadata{}
	}
	return document
}

func HashSpec(spec flowengine.FlowSpec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
