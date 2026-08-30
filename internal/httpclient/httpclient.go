package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/time/rate"

	"github.com/rawizhere/gosift/internal/config"
)

type Client struct {
	*retryablehttp.Client
	limiter *rate.Limiter
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
	if cfg.UserAgent != "" {
		ua := cfg.UserAgent
		rc.RequestLogHook = func(_ retryablehttp.Logger, req *http.Request, _ int) {
			req.Header.Set("User-Agent", ua)
		}
	}
	limiter := rate.NewLimiter(rate.Limit(cfg.StoreRPS), 1)
	return &Client{Client: rc, limiter: limiter}, nil
}

func (c *Client) Do(req *retryablehttp.Request) (*http.Response, error) {
	if err := c.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}
	return c.Client.Do(req)
}

func (c *Client) Request(ctx context.Context, method, url string) (*retryablehttp.Request, error) {
	return retryablehttp.NewRequestWithContext(ctx, method, url, nil)
}
