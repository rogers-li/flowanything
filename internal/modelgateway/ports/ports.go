package ports

import (
	"context"

	"flow-anything/internal/platform/contracts/model"
)

type ChatProvider interface {
	Chat(ctx context.Context, req model.ChatRequest) (model.ChatResponse, error)
}

type ChatProviderMetadata struct {
	Name         string
	BaseURL      string
	DefaultModel string
	Attributes   map[string]string
}

type ChatProviderMetadataProvider interface {
	ChatProviderMetadata() ChatProviderMetadata
}

func DescribeChatProvider(provider ChatProvider) ChatProviderMetadata {
	if provider == nil {
		return ChatProviderMetadata{Name: "unconfigured"}
	}
	if metadataProvider, ok := provider.(ChatProviderMetadataProvider); ok {
		metadata := metadataProvider.ChatProviderMetadata()
		if metadata.Name == "" {
			metadata.Name = "unknown"
		}
		return metadata
	}
	return ChatProviderMetadata{Name: "unknown"}
}
