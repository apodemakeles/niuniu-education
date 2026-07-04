package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/apodemakeles/niuniu-education/backend/internal/config"
	"github.com/apodemakeles/niuniu-education/backend/internal/module/wordlibrary"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/fs"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/ocr"
	"github.com/apodemakeles/niuniu-education/backend/internal/platform/storage"
	"github.com/apodemakeles/niuniu-education/backend/internal/server"
	"github.com/apodemakeles/niuniu-education/backend/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "启动失败: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 启动参数（优先级最高）
	dataDirFlag := flag.String("data-dir", "", "数据目录（覆盖配置文件）")
	portFlag := flag.Int("port", 0, "API 服务端口（覆盖配置文件）")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 1. 解析数据目录：参数 > 默认。配置文件位于 <dataDir>/config.yaml。
	dataDir := *dataDirFlag
	if dataDir == "" {
		dataDir = "./data"
	}
	cfgPath := filepath.Join(dataDir, "config.yaml")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	// 启动参数覆盖
	if *dataDirFlag != "" {
		cfg.DataDir = *dataDirFlag
	}
	if *portFlag > 0 {
		cfg.Server.Port = *portFlag
	}

	// 2. 数据目录初始化与校验
	absDataDir, err := fs.EnsureDataDir(cfg)
	if err != nil {
		return err
	}
	// 配置文件不存在则生成一份默认值
	if err := config.SaveIfAbsent(cfgPath, cfg); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}

	logger.Info("配置加载完成",
		slog.String("dataDir", absDataDir),
		slog.String("configFile", cfgPath),
		slog.Int("port", cfg.Server.Port),
		slog.String("ocrProvider", cfg.OCR.Provider),
		slog.String("version", version.Version),
	)

	// 3. 数据库（含迁移）
	ctx := context.Background()
	db, err := storage.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	// 4. OCR Provider（mock 离线可用；deepseek 需要 apiKey，缺失时调用报错不阻塞启动）
	ocrProvider, err := ocr.New(cfg.OCR)
	if err != nil {
		return fmt.Errorf("init ocr provider: %w", err)
	}
	logger.Info("OCR provider 就绪", slog.String("provider", ocrProvider.Name()))

	// 5. 业务模块装配
	store := wordlibrary.NewStore(db)
	svc := wordlibrary.NewService(store, ocrProvider, logger)
	wlHandler := wordlibrary.NewHandler(svc, store, db, cfg, logger)

	// 6. HTTP 服务
	srv := server.New(cfg, logger, func(r server.SubRouter) {
		wlHandler.Register(r)
	})
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 180 * time.Second, // OCR 较慢
		IdleTimeout:  120 * time.Second,
	}

	// 5. 优雅启停
	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务启动", slog.String("addr", httpSrv.Addr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("http serve: %w", err)
	case <-stopCh:
		logger.Info("收到退出信号，正在关闭...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	logger.Info("已关闭")
	return nil
}
