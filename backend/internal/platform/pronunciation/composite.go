package pronunciation

import (
	"context"
	"errors"
	"fmt"
)

type CompositeProvider struct {
	providers []Provider
}

func NewCompositeProvider(providers ...Provider) *CompositeProvider {
	filtered := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		if provider != nil {
			filtered = append(filtered, provider)
		}
	}
	return &CompositeProvider{providers: filtered}
}

func (p *CompositeProvider) Name() string { return "composite" }

func (p *CompositeProvider) Lookup(ctx context.Context, query Query) (Result, error) {
	var upstreamErrors []error
	for _, provider := range p.providers {
		result, err := provider.Lookup(ctx, query)
		switch {
		case err == nil:
			if result.Provider == "" {
				result.Provider = provider.Name()
			}
			return result, nil
		case errors.Is(err, ErrNotFound):
			continue
		default:
			// 某一路 429/5xx 不阻断后续独立来源；全部失败时再汇总返回。
			upstreamErrors = append(upstreamErrors, err)
		}
	}
	if len(upstreamErrors) > 0 {
		return Result{}, fmt.Errorf("所有发音来源均失败: %w", errors.Join(upstreamErrors...))
	}
	return Result{}, ErrNotFound
}
