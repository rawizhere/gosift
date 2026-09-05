// Package httpclient provides a store-parser HTTP client with browser TLS fingerprint impersonation, rate limiting and retries.
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	nethttp "net/http"
	"sort"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"golang.org/x/time/rate"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/randutil"
)

// maxImageBytes caps a single downloaded image.
const maxImageBytes = 10 << 20

// fingerprint pairs a browser TLS/HTTP2 profile with a matching User-Agent; a mismatch would be a bot signal.
type fingerprint struct {
	name      string
	profile   profiles.ClientProfile
	userAgent string
}

// fingerprintPool holds desktop and mobile browser fingerprints.
var fingerprintPool = []fingerprint{
	{
		name:      "chrome_152_win",
		profile:   profiles.Chrome_152,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36",
	},
	{
		name:      "chrome_150_win",
		profile:   profiles.Chrome_150,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
	},
	{
		name:      "firefox_148_mac",
		profile:   profiles.Firefox_148,
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:148.0) Gecko/20100101 Firefox/148.0",
	},
	{
		name:      "safari_ios_18_5",
		profile:   profiles.Safari_IOS_18_5,
		userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1",
	},
	{
		name:      "safari_16_mac",
		profile:   profiles.Safari_16_0,
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Safari/605.1.15",
	},
}

// pickFingerprint returns a random fingerprint from the pool.
func pickFingerprint() fingerprint {
	return fingerprintPool[rand.IntN(len(fingerprintPool))]
}

type Client struct {
	mu        sync.Mutex
	tc        tls_client.HttpClient
	ua        string
	fp        fingerprint
	rotatedAt time.Time
	cfg       *config.Config
	log       *slog.Logger
	limit     map[string]*rate.Limiter
}

// roundTripper adapts the impersonation client for the Telegram API http.Client.
type roundTripper struct {
	c *Client
}

func (rt roundTripper) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) {
	return rt.c.send(req)
}

// New creates a client with a random browser fingerprint, optional proxy and per-store rate limiting.
func New(cfg *config.Config, log *slog.Logger) (*Client, error) {
	fp := pickFingerprint()
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(fp.profile),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
	}
	if cfg.ParseTimeout > 0 {
		opts = append(opts, tls_client.WithTimeoutSeconds(int(cfg.ParseTimeout.Seconds())+1))
	}
	if cfg.HTTPProxy != "" {
		opts = append(opts, tls_client.WithProxyUrl(cfg.HTTPProxy))
	}
	tc, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return nil, fmt.Errorf("tls client: %w", err)
	}
	c := &Client{tc: tc, ua: fp.userAgent, fp: fp, cfg: cfg, log: log, rotatedAt: time.Now(), limit: map[string]*rate.Limiter{}}
	return c, nil
}

// StandardClient returns a plain http.Client for the Telegram API without per-store rate limiting.
func (c *Client) StandardClient() *nethttp.Client {
	timeout := c.cfg.ParseTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &nethttp.Client{Transport: roundTripper{c: c}, Timeout: timeout}
}

func (c *Client) limiter(store string) *rate.Limiter {
	l, ok := c.limit[store]
	if !ok {
		rps := c.cfg.StoreRPS
		if rps <= 0 {
			rps = 0.5
		}
		l = rate.NewLimiter(rate.Limit(rps), 1)
		c.limit[store] = l
	}
	return l
}

// maybeRotate switches to a fresh fingerprint once the current one outlives the parse interval.
func (c *Client) maybeRotate() {
	rotateEvery := c.cfg.ParseInterval
	if rotateEvery <= 0 {
		rotateEvery = 15 * time.Minute
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.rotatedAt) < rotateEvery {
		return
	}
	fp := pickFingerprint()
	for fp.name == c.fp.name {
		fp = pickFingerprint()
	}
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(fp.profile),
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
	}
	if c.cfg.ParseTimeout > 0 {
		opts = append(opts, tls_client.WithTimeoutSeconds(int(c.cfg.ParseTimeout.Seconds())+1))
	}
	if c.cfg.HTTPProxy != "" {
		opts = append(opts, tls_client.WithProxyUrl(c.cfg.HTTPProxy))
	}
	tc, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return // keep the current fingerprint on rebuild failure
	}
	c.tc.CloseIdleConnections()
	c.tc = tc
	c.fp = fp
	c.ua = fp.userAgent
	c.rotatedAt = time.Now()
	c.log.Info("fingerprint rotated", "profile", fp.name)
}

// shouldRetry reports whether a status is worth retrying: 429, 5xx except 501 and, for API calls, 404.
// A 404 can come from a broken load-balancer node, so the retry must dial a fresh connection.
func shouldRetry(status int, retryNotFound bool) bool {
	if retryNotFound && status == nethttp.StatusNotFound {
		return true
	}
	return status == nethttp.StatusTooManyRequests || (status >= 500 && status != nethttp.StatusNotImplemented)
}

// backoff returns the wait before the given retry attempt.
func (c *Client) backoff(attempt int) time.Duration {
	floor := c.cfg.ParseRetryBackoff
	if floor <= 0 {
		floor = 5 * time.Second
	}
	ceil := 5 * floor
	wait := floor << attempt
	if wait > ceil || wait <= 0 {
		wait = ceil
	}
	return wait + randutil.Duration(wait)
}

// Do sends the request with per-store rate limiting and retries on transport errors, 404, 429 and 5xx except 501.
func (c *Client) Do(ctx context.Context, req *nethttp.Request, store string) (*nethttp.Response, error) {
	c.maybeRotate()
	if err := c.limiter(store).Wait(ctx); err != nil {
		return nil, err
	}
	return c.doWithRetries(ctx, req, true)
}

// doWithRetries sends the request up to ParseRetries+1 times, recycling connections between attempts.
func (c *Client) doWithRetries(ctx context.Context, req *nethttp.Request, retryNotFound bool) (*nethttp.Response, error) {
	retries := c.cfg.ParseRetries
	if retries <= 0 {
		retries = 3
	}
	var resp *nethttp.Response
	var err error
	for attempt := 0; ; attempt++ {
		resp, err = c.send(req.WithContext(ctx))
		if err == nil && !shouldRetry(resp.StatusCode, retryNotFound) {
			return resp, nil
		}
		if attempt >= retries {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			c.log.Warn("request retry", "attempt", attempt+1, "status", resp.StatusCode)
		} else {
			c.log.Warn("request retry", "attempt", attempt+1, "error", err)
		}
		c.mu.Lock()
		c.tc.CloseIdleConnections()
		c.mu.Unlock()
		wait := c.backoff(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// send converts the request to fhttp for tls-client and converts the response back.
func (c *Client) send(req *nethttp.Request) (*nethttp.Response, error) {
	c.mu.Lock()
	tc, ua := c.tc, c.ua
	c.mu.Unlock()
	freq, err := toFHTTP(req, ua)
	if err != nil {
		return nil, err
	}
	fresp, err := tc.Do(freq)
	if err != nil {
		return nil, err
	}
	return fromFHTTP(fresp, req), nil
}

// toFHTTP builds an fhttp request from a net/http request with a buffered body.
func toFHTTP(req *nethttp.Request, ua string) (*fhttp.Request, error) {
	var body io.Reader
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		_ = req.Body.Close()
		body = bytes.NewReader(data)
	}
	freq, err := fhttp.NewRequest(req.Method, req.URL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	freq = freq.WithContext(req.Context())
	for key, vals := range req.Header {
		for _, v := range vals {
			freq.Header.Add(key, v)
		}
	}
	if ua != "" && req.Header.Get("User-Agent") == "" {
		freq.Header.Set("User-Agent", ua)
	}
	// Send user-agent first so the header order matches real browsers.
	order := []string{"user-agent"}
	for _, key := range keysSorted(freq.Header) {
		lower := toLowerASCII(key)
		if lower != "user-agent" && lower != fhttp.HeaderOrderKey && lower != fhttp.PHeaderOrderKey {
			order = append(order, lower)
		}
	}
	freq.Header[fhttp.HeaderOrderKey] = order
	if req.Host != "" {
		freq.Host = req.Host
	}
	return freq, nil
}

// fromFHTTP converts a tls-client response back to net/http.
func fromFHTTP(fresp *fhttp.Response, req *nethttp.Request) *nethttp.Response {
	resp := &nethttp.Response{
		Status:        fresp.Status,
		StatusCode:    fresp.StatusCode,
		Proto:         fresp.Proto,
		ProtoMajor:    fresp.ProtoMajor,
		ProtoMinor:    fresp.ProtoMinor,
		Header:        nethttp.Header{},
		ContentLength: fresp.ContentLength,
		Body:          io.NopCloser(fresp.Body),
		Request:       req,
	}
	for key, vals := range fresp.Header {
		if key == fhttp.HeaderOrderKey || key == fhttp.PHeaderOrderKey {
			continue
		}
		for _, v := range vals {
			resp.Header.Add(key, v)
		}
	}
	return resp
}

func keysSorted(h fhttp.Header) []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toLowerASCII(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}

// GetBytes downloads a small payload without the per-store rate limiter; 404 is not retried.
func (c *Client) GetBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.doWithRetries(ctx, req, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != nethttp.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", rawURL, err)
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("get %s: payload too large", rawURL)
	}
	return data, nil
}
