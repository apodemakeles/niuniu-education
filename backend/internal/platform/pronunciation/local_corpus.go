package pronunciation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// LocalCorpusProvider 基于本地离线真人发音库查找音频。
// 仓库结构为扁平的 {word}.mp3 文件目录，文件名均为小写。
// 同一个类型服务 Cambridge 和 TFD 两个语料库，仅 provider 名和目录不同。
type LocalCorpusProvider struct {
	name string
	dir  string
}

// NewLocalCorpusProvider 创建一个本地语料库 provider。
// name 为 provider 标识（如 "cambridge"、"tfd"）；dir 为对应子目录的绝对路径。
func NewLocalCorpusProvider(name, dir string) *LocalCorpusProvider {
	return &LocalCorpusProvider{name: name, dir: dir}
}

func (p *LocalCorpusProvider) Name() string { return p.name }

func (p *LocalCorpusProvider) Lookup(ctx context.Context, query Query) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	// 离线库文件名约定为 {word小写}.mp3。
	// query.Text 为单词原文，统一转小写匹配。
	word := strings.ToLower(strings.TrimSpace(query.Text))
	if word == "" {
		return Result{}, ErrNotFound
	}
	path := filepath.Join(p.dir, word+".mp3")

	info, err := os.Stat(path)
	if err != nil {
		// 文件不存在（含目录未配置的情况），视为该来源没有此词，交给后续 provider。
		return Result{}, ErrNotFound
	}
	if info.IsDir() || info.Size() < 1024 {
		return Result{}, ErrNotFound
	}

	// AudioURL 填 file:// 形式，便于日志/调试可读；FilePath 为 Service 实际拷贝用的绝对路径。
	// 在线 provider 的 AudioURL 用于 HTTP 下载；离线 provider 改用 FilePath 直接拷贝，AudioURL 仅作来源记录。
	return Result{
		Provider:    p.name,
		Locale:      query.Locale,
		AudioURL:    "file://" + path,
		FilePath:    path,
		LicenseName: licenseName(p.name),
		SourceURL:   "file://" + path,
	}, nil
}

func licenseName(provider string) string {
	switch provider {
	case "cambridge":
		return "Cambridge Dictionary (offline corpus)"
	case "tfd":
		return "The Free Dictionary (offline corpus)"
	default:
		return "offline corpus"
	}
}
