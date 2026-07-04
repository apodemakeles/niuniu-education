package fs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
)

// EnsureDataDir 校验并初始化数据目录及其子目录（images/、exports/）。
// 目录不存在则创建；不可写则返回明确错误。
func EnsureDataDir(cfg *config.Config) (string, error) {
	abs, err := cfg.AbsDataDir()
	if err != nil {
		return "", err
	}
	for _, sub := range []string{abs, filepath.Join(abs, "images"), filepath.Join(abs, "exports")} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return "", fmt.Errorf("create dir %s: %w", sub, err)
		}
	}
	// 可写性校验
	testFile := filepath.Join(abs, ".write_test")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		return "", fmt.Errorf("dataDir %s 不可写: %w", abs, err)
	}
	_ = os.Remove(testFile)
	return abs, nil
}
