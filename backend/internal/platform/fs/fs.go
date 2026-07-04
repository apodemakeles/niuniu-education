package fs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// SaveImage 把图片字节保存到 data/images/YYYY/MM/<hexid>.<ext>。
// 返回相对 data 目录的路径（用于存入 import_images.file_path）。
func SaveImage(cfg *config.Config, data []byte, mimeType string) (relPath string, err error) {
	abs, err := cfg.AbsDataDir()
	if err != nil {
		return "", err
	}
	now := time.Now()
	subDir := filepath.Join("images", now.Format("2006/01"))
	fullDir := filepath.Join(abs, subDir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", fmt.Errorf("create image dir: %w", err)
	}
	name := randomHex(8) + extFromMime(mimeType)
	relPath = filepath.Join(subDir, name)
	if err := os.WriteFile(filepath.Join(abs, relPath), data, 0o644); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}
	return relPath, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func extFromMime(mt string) string {
	switch strings.ToLower(strings.TrimSpace(mt)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".img"
	}
}
