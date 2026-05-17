package agentcore

func assembleContextMessages(ctx Context, runtime StrategyRuntime, req AgentRunRequest, strategy string, phase ContextPhase, systemPrompt string, observations []ActionResult, extra []Message) ([]Message, error) {
	assembler := runtime.ContextAssembler
	if assembler == nil {
		assembler = NewDefaultContextAssembler()
	}
	memories, err := recallMemories(ctx, runtime.Memory, req)
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategy, EventContextFailed, map[string]any{"phase": phase}, err.Error()))
		return nil, err
	}
	assembly, err := assembler.Assemble(ctx, ContextAssemblyRequest{
		Agent:         req.Agent,
		Phase:         phase,
		SystemPrompt:  systemPrompt,
		UserMessage:   req.UserMessage,
		Conversation:  req.Conversation,
		Context:       req.Context,
		Memories:      memories,
		Observations:  observations,
		ExtraMessages: extra,
	})
	if err != nil {
		runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategy, EventContextFailed, map[string]any{"phase": phase}, err.Error()))
		return nil, err
	}
	runtime.Events.PublishAgentEvent(ctx, strategyEvent(req, strategy, EventContextAssembled, map[string]any{
		"phase":  phase,
		"report": assembly.Report,
	}, ""))
	return assembly.Messages, nil
}
