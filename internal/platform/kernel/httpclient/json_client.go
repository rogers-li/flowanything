package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	return NewWithHTTPClient(baseURL, &http.Client{
		Timeout: timeout,
	})
}

func NewWithHTTPClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: client,
	}
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(path, query), nil)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build http request", err)
	}

	return c.do(req, out)
}

func (c *Client) PostJSON(ctx context.Context, path string, in any, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode json request", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path, nil), bytes.NewReader(body))
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build http request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, "http request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return decodeError(resp)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to decode json response", err)
	}

	return nil
}

func (c *Client) url(path string, query url.Values) string {
	if query == nil || len(query) == 0 {
		return c.baseURL + "/" + strings.TrimLeft(path, "/")
	}

	return c.baseURL + "/" + strings.TrimLeft(path, "/") + "?" + query.Encode()
}

func decodeError(resp *http.Response) error {
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.Error.Code != "" {
		return apperrors.New(apperrors.Code(body.Error.Code), body.Error.Message)
	}

	return apperrors.New(
		codeFromStatus(resp.StatusCode),
		fmt.Sprintf("http request failed with status %d", resp.StatusCode),
	)
}

func codeFromStatus(statusCode int) apperrors.Code {
	switch statusCode {
	case http.StatusBadRequest:
		return apperrors.CodeInvalidArgument
	case http.StatusUnauthorized:
		return apperrors.CodeUnauthorized
	case http.StatusForbidden:
		return apperrors.CodeForbidden
	case http.StatusNotFound:
		return apperrors.CodeNotFound
	case http.StatusConflict:
		return apperrors.CodeConflict
	case http.StatusServiceUnavailable:
		return apperrors.CodeUnavailable
	default:
		return apperrors.CodeInternal
	}
}
