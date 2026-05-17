package domain

import (
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type ConversationRef struct {
	TenantID  tenant.ID
	AgentID   id.ID
	SessionID id.ID
}

func NewConversationRef(tenantID tenant.ID, agentID id.ID, sessionID id.ID) (ConversationRef, bool) {
	if tenantID.Empty() || agentID.Empty() || sessionID.Empty() {
		return ConversationRef{}, false
	}

	return ConversationRef{
		TenantID:  tenantID,
		AgentID:   agentID,
		SessionID: sessionID,
	}, true
}

func (r ConversationRef) Key() string {
	return r.TenantID.String() + "/" + r.AgentID.String() + "/" + r.SessionID.String()
}
