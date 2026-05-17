package config

type DiagnosticSeverity string

const (
	DiagnosticError   DiagnosticSeverity = "error"
	DiagnosticWarning DiagnosticSeverity = "warning"
)

type Diagnostic struct {
	Severity DiagnosticSeverity `json:"severity"`
	Path     string             `json:"path"`
	Message  string             `json:"message"`
}

type DependencyEdge struct {
	From     ResourceRef `json:"from"`
	To       ResourceRef `json:"to"`
	Optional bool        `json:"optional"`
}

type EditorState struct {
	Bundle       BundleSpec           `json:"bundle"`
	Resources    []ResourceDescriptor `json:"resources"`
	Dependencies []DependencyEdge     `json:"dependencies"`
	Diagnostics  []Diagnostic         `json:"diagnostics"`
}

// InspectBundle gives Console and other config editors a read-optimized view
// over a bundle: normalized config, indexed resources, dependency edges, and
// validation diagnostics. It never mutates or persists configuration.
func InspectBundle(bundle BundleSpec) EditorState {
	normalized := NormalizeBundle(bundle)
	state := EditorState{
		Bundle:       normalized,
		Dependencies: ExtractDependencyEdges(normalized),
	}
	index, indexErr := BuildIndex(normalized)
	if indexErr == nil {
		state.Resources = index.ListDescriptors()
	}
	if err := ValidateBundle(normalized); err != nil {
		state.Diagnostics = append(state.Diagnostics, diagnosticsFromError(err)...)
	}
	if indexErr != nil && len(state.Diagnostics) == 0 {
		state.Diagnostics = append(state.Diagnostics, diagnosticsFromError(indexErr)...)
	}
	return state
}

func ExtractDependencyEdges(bundle BundleSpec) []DependencyEdge {
	edges := []DependencyEdge{}
	bundleRef := ResourceRef{Kind: ResourceKind(BundleKind), ID: bundle.ID}
	for _, dependency := range bundle.Dependencies {
		edges = append(edges, DependencyEdge{From: bundleRef, To: dependency, Optional: dependency.Optional})
	}
	for _, agent := range bundle.Resources.Agents {
		from := ResourceRef{Kind: ResourceAgent, ID: agent.ID}
		if agent.ModelRef.ID != "" {
			edges = append(edges, DependencyEdge{From: from, To: withDefaultKind(agent.ModelRef, ResourceModel), Optional: agent.ModelRef.Optional})
		}
		edges = append(edges, bindingEdges(from, agent.Skills, ResourceSkill)...)
		edges = append(edges, bindingEdges(from, agent.Tools, ResourceTool)...)
		edges = append(edges, bindingEdges(from, agent.Workflows, ResourceWorkflow)...)
		edges = append(edges, bindingEdges(from, agent.Knowledge, ResourceKnowledge)...)
		edges = append(edges, refEdges(from, agent.Policies, ResourcePolicy)...)
	}
	for _, skill := range bundle.Resources.Skills {
		from := ResourceRef{Kind: ResourceSkill, ID: skill.ID}
		edges = append(edges, bindingEdges(from, skill.Tools, ResourceTool)...)
		edges = append(edges, bindingEdges(from, skill.Knowledge, ResourceKnowledge)...)
		edges = append(edges, refEdges(from, skill.Policies, ResourcePolicy)...)
	}
	for _, tool := range bundle.Resources.Tools {
		from := ResourceRef{Kind: ResourceTool, ID: tool.ID}
		if tool.Implementation.Ref.ID != "" {
			edges = append(edges, DependencyEdge{From: from, To: tool.Implementation.Ref, Optional: tool.Implementation.Ref.Optional})
		}
	}
	for _, knowledge := range bundle.Resources.KnowledgeBases {
		from := ResourceRef{Kind: ResourceKnowledge, ID: knowledge.ID}
		if knowledge.EmbeddingModelRef.ID != "" {
			edges = append(edges, DependencyEdge{From: from, To: withDefaultKind(knowledge.EmbeddingModelRef, ResourceModel), Optional: knowledge.EmbeddingModelRef.Optional})
		}
	}
	return edges
}

func bindingEdges(from ResourceRef, bindings []ResourceBinding, defaultKind ResourceKind) []DependencyEdge {
	edges := []DependencyEdge{}
	for _, binding := range bindings {
		if binding.Disabled || binding.Ref.ID == "" {
			continue
		}
		ref := withDefaultKind(binding.Ref, defaultKind)
		edges = append(edges, DependencyEdge{From: from, To: ref, Optional: ref.Optional})
	}
	return edges
}

func refEdges(from ResourceRef, refs []ResourceRef, defaultKind ResourceKind) []DependencyEdge {
	edges := []DependencyEdge{}
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		ref = withDefaultKind(ref, defaultKind)
		edges = append(edges, DependencyEdge{From: from, To: ref, Optional: ref.Optional})
	}
	return edges
}

func withDefaultKind(ref ResourceRef, kind ResourceKind) ResourceRef {
	if ref.Kind == "" {
		ref.Kind = kind
	}
	return ref
}

func diagnosticsFromError(err error) []Diagnostic {
	if err == nil {
		return nil
	}
	if validationErrors, ok := err.(ValidationErrors); ok {
		out := make([]Diagnostic, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			out = append(out, Diagnostic{
				Severity: DiagnosticError,
				Path:     validationError.Path,
				Message:  validationError.Message,
			})
		}
		return out
	}
	return []Diagnostic{{Severity: DiagnosticError, Message: err.Error()}}
}
