// Package pronunciation 提供统一的单词发音来源抽象。
// 录音词典、Wiktionary 和未来的 TTS 都实现 Provider，由 CompositeProvider 依优先级组合。
package pronunciation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("未找到符合条件的发音")

type Query struct {
	Text     string
	Locale   string
	Phonetic string
}

type Result struct {
	Provider    string
	Locale      string
	Phonetic    string
	AudioURL    string
	SourceURL   string
	LicenseName string
	LicenseURL  string
	Attribution string
}

type Provider interface {
	Name() string
	Lookup(ctx context.Context, query Query) (Result, error)
}

// UpstreamError 表示上游暂时不可用；RetryAfter 可透传给 HTTP 客户端。
type UpstreamError struct {
	Provider   string
	StatusCode int
	RetryAfter time.Duration
	Cause      error
}

func (e *UpstreamError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s 上游请求失败（%d）：%v", e.Provider, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("%s 上游请求失败（%d）", e.Provider, e.StatusCode)
}

func (e *UpstreamError) Unwrap() error { return e.Cause }
