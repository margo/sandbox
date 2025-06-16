package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type HTTP1Transport struct {
	client  *http.Client
	baseURL string
	timeout time.Duration
}

func NewHTTP1Transport(baseURL string, timeout time.Duration) *HTTP1Transport {
	return &HTTP1Transport{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		baseURL: baseURL,
		timeout: timeout,
	}
}

func (h *HTTP1Transport) Send(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, h.baseURL+req.Path, req.Body)
	if err != nil {
		return nil, err
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       resp.Body,
	}, nil
}

func (h *HTTP1Transport) Stream(ctx context.Context) (Stream, error) {
	return nil, fmt.Errorf("streaming not supported for HTTP/1.1")
}

func (h *HTTP1Transport) Close() error {
	h.client.CloseIdleConnections()
	return nil
}

func (h *HTTP1Transport) Protocol() ProtocolType {
	return HTTP1
}
