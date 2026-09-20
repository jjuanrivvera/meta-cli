package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Object map[string]any

func (client *Client) Read(ctx context.Context, requestPath string, query url.Values) (any, error) {
	return client.operation(ctx, Request{Method: http.MethodGet, Path: requestPath, Query: query})
}

func (client *Client) Write(ctx context.Context, requestPath string, query url.Values, body []byte) (any, error) {
	return client.operation(ctx, Request{Method: http.MethodPost, Path: requestPath, Query: query, Body: body})
}

func (client *Client) Remove(ctx context.Context, requestPath string, query url.Values) (any, error) {
	return client.operation(ctx, Request{Method: http.MethodDelete, Path: requestPath, Query: query})
}

func (client *Client) operation(ctx context.Context, request Request) (any, error) {
	response, err := client.Do(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(response.Body) == 0 {
		return Object{"success": true}, nil
	}
	var result any
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, fmt.Errorf("decode Graph response: %w", err)
	}
	return result, nil
}
