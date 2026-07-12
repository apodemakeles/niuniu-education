package pronunciation

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RetryingClient 为无鉴权的上游词典提供共享限速和 429/5xx 重试。
type RetryingClient struct {
	Client      HTTPDoer
	Interval    time.Duration
	MaxRetries  int
	BaseBackoff time.Duration

	mu          sync.Mutex
	nextAllowed time.Time
}

func NewRetryingClient(client HTTPDoer, interval time.Duration, maxRetries int) *RetryingClient {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		// 某些本地 Go 运行环境不会稳定采用 ProxyFromEnvironment；发音上游请求
		// 显式读取代理，确保与 curl/浏览器的网络路径一致。
		if proxyURL := firstEnv("HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"); proxyURL != "" {
			if parsed, err := url.Parse(proxyURL); err == nil {
				transport.Proxy = http.ProxyURL(parsed)
			}
		}
		client = &http.Client{Timeout: 25 * time.Second, Transport: transport}
	}
	return &RetryingClient{Client: client, Interval: interval, MaxRetries: maxRetries, BaseBackoff: 500 * time.Millisecond}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (c *RetryingClient) Do(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := c.waitTurn(req.Context()); err != nil {
			return nil, err
		}
		resp, err := c.Client.Do(req.Clone(req.Context()))
		if err != nil {
			if attempt >= c.MaxRetries {
				return nil, err
			}
			if err := waitContext(req.Context(), c.backoff(attempt, "")); err != nil {
				return nil, err
			}
			continue
		}
		if !retryableStatus(resp.StatusCode) || attempt >= c.MaxRetries {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
		if err := waitContext(req.Context(), c.backoff(attempt, resp.Header.Get("Retry-After"))); err != nil {
			return nil, err
		}
	}
}

func (c *RetryingClient) waitTurn(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	wait := time.Duration(0)
	if now.Before(c.nextAllowed) {
		wait = c.nextAllowed.Sub(now)
	}
	c.nextAllowed = now.Add(wait + c.Interval)
	c.mu.Unlock()
	return waitContext(ctx, wait)
}

func (c *RetryingClient) backoff(attempt int, retryAfter string) time.Duration {
	if d := parseRetryAfter(retryAfter); d > 0 {
		return d
	}
	base := c.BaseBackoff
	if base <= 0 {
		base = 500 * time.Millisecond
	}
	return base * time.Duration(1<<attempt)
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
