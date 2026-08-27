package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/margo/sandbox/mis/pkg/types"
)

// Client communicates with the MIS HTTP server over a Unix domain socket.
type Client struct {
	socketPath string
	http       *http.Client
	logger     *slog.Logger
}

// New creates a Client that dials the given Unix socket path.
func New(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}

	return &Client{
		socketPath: socketPath,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// MintX509SVID sends a POST /mint/svid/x509 request and returns the SVID response.
func (c *Client) MintX509SVID(
	ctx context.Context,
	req *types.MintSVIDRequest,
) (*types.MintSVIDResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// The host value is arbitrary for Unix socket connections;
	// the transport ignores it and dials the socket path instead.
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://unix/mint/svid/x509",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp types.ErrorResponse
		if jsonErr := json.NewDecoder(resp.Body).Decode(&errResp); jsonErr != nil {
			return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf(
			"server error (%d): %s — %s",
			resp.StatusCode,
			errResp.Error,
			errResp.Details,
		)
	}

	var result types.MintSVIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
