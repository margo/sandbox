package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/margo/sandbox/mis/pkg/types"
)

// Client communicates with the MIS HTTP server over a Unix domain socket.
type Client struct {
	socketPath string
	http       *http.Client
	logger     *log.Logger
}

// New creates a Client that dials the given Unix socket path.
func New(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}

	c := &Client{
		socketPath: socketPath,
		logger:     log.New(os.Stdout, "", log.LstdFlags),
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}

	c.logger.Printf("[INFO] Client created for Unix socket path: %s", socketPath)
	return c
}

// MintX509SVID sends a POST /mint/svid/x509 request and returns the SVID response.
func (c *Client) MintX509SVID(
	ctx context.Context,
	req *types.MintSVIDRequest,
) (*types.MintSVIDResponse, error) {
	c.logger.Printf("[INFO] MintX509SVID: marshalling request")
	body, err := json.Marshal(req)
	if err != nil {
		c.logger.Printf("[ERROR] MintX509SVID: failed to marshal request: %v", err)
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
		c.logger.Printf("[ERROR] MintX509SVID: failed to build HTTP request: %v", err)
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.Printf("[DEBUG] MintX509SVID: sending POST request to http://unix/mint/svid/x509")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		c.logger.Printf("[ERROR] MintX509SVID: HTTP request failed: %v", err)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	c.logger.Printf("[DEBUG] MintX509SVID: received response with status code %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		var errResp types.ErrorResponse
		if jsonErr := json.NewDecoder(resp.Body).Decode(&errResp); jsonErr != nil {
			c.logger.Printf(
				"[ERROR] MintX509SVID: unexpected status %d and failed to decode error response: %v",
				resp.StatusCode,
				jsonErr,
			)
			return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
		}
		c.logger.Printf(
			"[ERROR] MintX509SVID: server returned error (%d): %s — %s",
			resp.StatusCode,
			errResp.Error,
			errResp.Details,
		)
		return nil, fmt.Errorf(
			"server error (%d): %s — %s",
			resp.StatusCode,
			errResp.Error,
			errResp.Details,
		)
	}

	var result types.MintSVIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Printf("[ERROR] MintX509SVID: failed to decode response body: %v", err)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	c.logger.Printf("[INFO] MintX509SVID: successfully minted X.509 SVID")
	return &result, nil
}
