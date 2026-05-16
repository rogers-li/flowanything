package application

import (
	"context"
	"log/slog"
	"time"

	"flow-anything/internal/modelgateway/domain"
	"flow-anything/internal/modelgateway/ports"
	"flow-anything/internal/platform/contracts/model"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
)

type Service struct {
	logger           *slog.Logger
	provider         ports.ChatProvider
	providerMetadata ports.ChatProviderMetadata
}

func New(logger *slog.Logger, provider ports.ChatProvider) *Service {
	metadata := ports.DescribeChatProvider(provider)
	logger.Info("model provider configured",
		"provider", metadata.Name,
		"provider_base_url", metadata.BaseURL,
		"default_model", metadata.DefaultModel,
		"provider_attributes", metadata.Attributes,
	)

	return &Service{
		logger:           logger,
		provider:         provider,
		providerMetadata: metadata,
	}
}

// Chat validates the platform-level chat request and delegates provider-specific
// protocol mapping to the configured model provider.
func (s *Service) Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error) {
	if req.ID.Empty() {
		req.ID = id.New("chatreq")
	}
	if err := domain.ValidateChatRequest(req); err != nil {
		return model.ChatResponse{}, err
	}
	if s.provider == nil {
		return model.ChatResponse{}, apperrors.New(apperrors.CodeUnavailable, "chat provider is not configured")
	}

	startedAt := time.Now()
	s.logger.Info("chat request started",
		"request_id", req.ID.String(),
		"trace_id", req.TraceID,
		"provider", s.providerMetadata.Name,
		"provider_base_url", s.providerMetadata.BaseURL,
		"default_model", s.providerMetadata.DefaultModel,
		"requested_model", req.Model,
		"message_count", len(req.Messages),
		"tool_count", len(req.Tools),
		"tool_choice", req.ToolChoice,
		"temperature", req.Options.Temperature,
		"max_tokens", req.Options.MaxTokens,
	)
	resp, err := s.provider.Chat(ctx, req)
	if err != nil {
		s.logger.Error("chat request failed",
			"request_id", req.ID.String(),
			"trace_id", req.TraceID,
			"provider", s.providerMetadata.Name,
			"provider_base_url", s.providerMetadata.BaseURL,
			"requested_model", req.Model,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return model.ChatResponse{}, err
	}
	if resp.Provider == "" {
		resp.Provider = s.providerMetadata.Name
	}
	if resp.ProviderURL == "" {
		resp.ProviderURL = s.providerMetadata.BaseURL
	}

	s.logger.Info("chat completed",
		"request_id", req.ID.String(),
		"response_id", resp.ID.String(),
		"trace_id", req.TraceID,
		"provider", s.providerMetadata.Name,
		"provider_base_url", s.providerMetadata.BaseURL,
		"requested_model", req.Model,
		"response_model", resp.Model,
		"finish_reason", resp.FinishReason,
		"tool_call_count", len(resp.Message.ToolCalls),
		"input_tokens", resp.Usage.InputTokens,
		"output_tokens", resp.Usage.OutputTokens,
		"total_tokens", resp.Usage.TotalTokens,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return resp, nil
}
