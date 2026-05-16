package infrastructure

import (
	"context"
	"net/url"
	"time"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/httpclient"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

type HTTPOperationRepository struct {
	client *httpclient.Client
}

func NewHTTPOperationRepository(platformAPIBaseURL string) *HTTPOperationRepository {
	return &HTTPOperationRepository{
		client: httpclient.New(platformAPIBaseURL, 10*time.Second),
	}
}

func (r *HTTPOperationRepository) GetOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error) {
	query := url.Values{}
	query.Set("tenant_id", tenantID.String())

	var spec connector.OperationSpec
	if err := r.client.GetJSON(ctx, "/v1/connector-operations/"+url.PathEscape(operationID.String()), query, &spec); err != nil {
		return connector.OperationSpec{}, err
	}

	return spec, nil
}
