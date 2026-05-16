package application

import (
	"flow-anything/internal/agentflow/domain"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type executionState struct {
	graph     domain.FlowGraph
	context   domain.RunContext
	scheduled map[id.ID]bool
	started   map[id.ID]bool
	completed map[id.ID]bool
}

func newExecutionState(graph domain.FlowGraph, context domain.RunContext) *executionState {
	return &executionState{
		graph:     graph,
		context:   context,
		scheduled: map[id.ID]bool{},
		started:   map[id.ID]bool{},
		completed: map[id.ID]bool{},
	}
}

func (s *executionState) schedule(nodeID id.ID) error {
	if nodeID.Empty() {
		return nil
	}
	if _, exists := s.graph.Nodes[nodeID]; !exists {
		return apperrors.New(apperrors.CodeInvalidArgument, "next node does not exist")
	}
	s.scheduled[nodeID] = true
	return nil
}

func (s *executionState) markStarted(nodeID id.ID) {
	s.started[nodeID] = true
}

func (s *executionState) complete(nodeID id.ID, result domain.NodeResult) {
	s.completed[nodeID] = true
	s.context = s.context.WithNodeResult(nodeID, result)
}

func (s *executionState) scheduledCount() int {
	return len(s.scheduled)
}

func (s *executionState) completedCount() int {
	return len(s.completed)
}

func (s *executionState) readyNodes() []id.ID {
	ready := make([]id.ID, 0)
	for nodeID := range s.scheduled {
		if s.started[nodeID] || s.completed[nodeID] {
			continue
		}
		if s.hasPendingScheduledUpstream(nodeID) {
			continue
		}
		ready = append(ready, nodeID)
	}
	return ready
}

func (s *executionState) hasPendingScheduledUpstream(nodeID id.ID) bool {
	for _, edge := range s.graph.Edges {
		if edge.ToNodeID != nodeID {
			continue
		}
		if s.scheduled[edge.FromNodeID] && !s.completed[edge.FromNodeID] {
			return true
		}
	}
	return false
}
