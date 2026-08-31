package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/time/rate"

	"github.com/rawizhere/gosift/internal/config"
)

// maxImageBytes caps a single downloaded image (Telegram allows 10 MB).
const maxImageBytes = 10 << 20

type Client struct {
	rc       *retryablehttp.Client
	rps      float64
	limiters map[string]*rate.Limiter
}

func NewRetryable(cfg *config.Config) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.HTTPProxy != "" {
		proxyURL, err := url.Parse(cfg.HTTPProxy)
		if err != nil {
			return nil, fmt.Errorf("parse http proxy: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	rc := retryablehttp.NewClient()
	rc.HTTPClient = &http.Client{Transport: transport, Timeout: cfg.ParseTimeout}
	rc.RetryMax = cfg.ParseRetries
	rc.RetryWaitMin = cfg.ParseRetryBackoff
	rc.RetryWaitMax = cfg.ParseRetryBackoff * 5
	rc.Logger = nil
	rc.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			return true, nil
		}
		return retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	}
	if cfg.UserAgent != "" {
		ua := cfg.UserAgent
		rc.RequestLogHook = func(_ retryablehttp.Logger, req *http.Request, _ int) {
			req.Header.Set("User-Agent", ua)
		}
	}
	return &Client{rc: rc, rps: cfg.StoreRPS, limiters: map[string]*rate.Limiter{}}, nil
}

func (c *Client) StandardClient() *http.Client {
	return c.rc.StandardClient()
}

func (c *Client) Do(ctx context.Context, req *retryablehttp.Request, store string) (*http.Response, error) {
	limiter := c.limiters[store]
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Limit(c.rps), 1)
		c.limiters[store] = limiter
	}
	if err := limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return c.rc.Do(req)
}

// GetBytes downloads a small payload (image, etc.) with retries but without the
// per-store rate limiter. Intended for static CDN content.
func (c *Client) GetBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.rc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("get %s: payload too large", url)
	}
	return data, nil
}
