package application

import (
	"strings"

	"flow-anything/internal/aiorchestrator/ports"
	"flow-anything/internal/contextengine"
)

const (
	DefaultSystemPrompt       = "你是企业 AI 中台中的智能助手。请用简洁、自然、可靠的中文回答用户。"
	DefaultMaxToolIterations  = 1
	DefaultMaxHistoryMessages = 20
	defaultFallbackReply      = "我已经收到你的请求。"
	defaultEmptyUserInputText = "用户发送了一条空消息。"
)

type runtimeOptions struct {
	DefaultSystemPrompt string
	MaxToolIterations   int
	MaxHistoryMessages  int
	ContextPolicy       contextengine.Policy
	ConversationStore   ports.ConversationStore
	TraceStore          ports.TraceStore
	RuntimeEventSink    ports.RuntimeEventSink
}

type Option func(*runtimeOptions)

func defaultRuntimeOptions() runtimeOptions {
	return runtimeOptions{
		DefaultSystemPrompt: DefaultSystemPrompt,
		MaxToolIterations:   DefaultMaxToolIterations,
		MaxHistoryMessages:  DefaultMaxHistoryMessages,
		ContextPolicy:       contextengine.DefaultPolicy(),
	}
}

// WithDefaultSystemPrompt configures the base system prompt used when no Agent
// specific prompt is available. This keeps deployment-level behavior out of the
// Orchestrator workflow code.
func WithDefaultSystemPrompt(prompt string) Option {
	return func(options *runtimeOptions) {
		if strings.TrimSpace(prompt) != "" {
			options.DefaultSystemPrompt = strings.TrimSpace(prompt)
		}
	}
}

// WithMaxToolIterations limits model-tool-model loops for one turn. The current
// implementation uses one iteration; the option makes the limit explicit for the
// upcoming state-machine based executor.
func WithMaxToolIterations(iterations int) Option {
	return func(options *runtimeOptions) {
		if iterations > 0 {
			options.MaxToolIterations = iterations
		}
	}
}

// WithMaxHistoryMessages bounds the short-term conversation context loaded for
// each model request. This is the first guardrail before adding token-aware
// context budgeting.
func WithMaxHistoryMessages(limit int) Option {
	return func(options *runtimeOptions) {
		if limit > 0 {
			options.MaxHistoryMessages = limit
			options.ContextPolicy.MaxHistoryMessages = limit
		}
	}
}

func WithContextPolicy(policy contextengine.Policy) Option {
	return func(options *runtimeOptions) {
		options.ContextPolicy = policy
		if options.ContextPolicy.MaxHistoryMessages <= 0 {
			options.ContextPolicy.MaxHistoryMessages = options.MaxHistoryMessages
		}
	}
}

func WithConversationStore(store ports.ConversationStore) Option {
	return func(options *runtimeOptions) {
		options.ConversationStore = store
	}
}

func WithTraceStore(store ports.TraceStore) Option {
	return func(options *runtimeOptions) {
		options.TraceStore = store
	}
}

func WithRuntimeEventSink(sink ports.RuntimeEventSink) Option {
	return func(options *runtimeOptions) {
		options.RuntimeEventSink = sink
	}
}
