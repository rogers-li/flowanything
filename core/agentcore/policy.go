package agentcore

const (
	defaultMaxIterations      = 3
	defaultMaxActions         = 8
	defaultMaxContextTokens   = 32000
	defaultMaxHistoryMessages = 20
	defaultMaxMemoryItems     = 8
	defaultMaxMessageChars    = 12000
)

func normalizeAgentPolicy(policy AgentPolicy) AgentPolicy {
	if policy.MaxIterations <= 0 {
		policy.MaxIterations = defaultMaxIterations
	}
	if policy.MaxActions <= 0 {
		policy.MaxActions = defaultMaxActions
	}
	if policy.MaxContextTokens <= 0 {
		policy.MaxContextTokens = defaultMaxContextTokens
	}
	if policy.MaxHistoryMessages <= 0 {
		policy.MaxHistoryMessages = defaultMaxHistoryMessages
	}
	if policy.MaxMemoryItems <= 0 {
		policy.MaxMemoryItems = defaultMaxMemoryItems
	}
	if policy.MaxMessageChars <= 0 {
		policy.MaxMessageChars = defaultMaxMessageChars
	}
	return policy
}
