package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func splitCSV(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Config 是应用运行配置，对应 data/config.yaml。
type Config struct {
	Server      ServerConfig `yaml:"server"`
	DataDir     string       `yaml:"dataDir"`
	Database    DBConfig     `yaml:"database"`
	OCR         OCRConfig    `yaml:"ocr"`
	OpenBrowser bool         `yaml:"openBrowser"`

	// ConfigFile 是配置文件自身的绝对路径，运行时填充，不写回 yaml。
	ConfigFile string `yaml:"-"`
}

type ServerConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowedOrigins"`
}

type DBConfig struct {
	JournalMode string `yaml:"journalMode"`
	BusyTimeout int    `yaml:"busyTimeout"`
}

type OCRConfig struct {
	Provider string `yaml:"provider"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	Model    string `yaml:"model"`
}

// Default 返回带默认值的配置。
func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8787,
			CORS: CORSConfig{
				Enabled: true,
				AllowedOrigins: []string{
					"http://localhost:5173",
					"http://localhost:4173",
					"http://localhost:8080",
				},
			},
		},
		DataDir: "./data",
		Database: DBConfig{
			JournalMode: "WAL",
			BusyTimeout: 5000,
		},
		OCR: OCRConfig{
			Provider: "siliconflow",
			Endpoint: "https://api.siliconflow.cn/v1",
			// 默认用 Qwen3-VL-32B：实测对竖版/复杂教材图识别更稳定。
			// DeepSeek-OCR 对部分图片返回乱码，已降级为备选。
			// 可在 data/config.yaml 改 model 为其他硅基流动视觉模型。
			Model:  "Qwen/Qwen3-VL-32B-Instruct",
			APIKey: "",
		},
		OpenBrowser: true,
	}
}

// Load 从 dataDir/config.yaml 读取配置，缺失则用默认值并落盘一份。
// 优先级：环境变量 > 配置文件 > 默认值。（启动参数在 main 中通过覆盖项处理）
func Load(cfgPath string) (*Config, error) {
	cfg := Default()
	cfg.ConfigFile = cfgPath

	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", cfgPath, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", cfgPath, err)
	}

	applyEnv(&cfg)

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveIfAbsent 在配置文件不存在时，用默认值生成一份，方便用户后续编辑。
func SaveIfAbsent(cfgPath string, cfg *Config) error {
	if _, err := os.Stat(cfgPath); err == nil {
		return nil // 已存在，不覆盖
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("NIUNIU_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("NIUNIU_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("NIUNIU_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("NIUNIU_CORS_ALLOWED_ORIGINS"); v != "" {
		// 逗号分隔的多个 origin，覆盖默认 allowedOrigins
		cfg.Server.CORS.AllowedOrigins = splitCSV(v)
	}
	if v := os.Getenv("NIUNIU_OCR_PROVIDER"); v != "" {
		cfg.OCR.Provider = v
	}
	if v := os.Getenv("NIUNIU_OCR_API_KEY"); v != "" {
		cfg.OCR.APIKey = v
	}
	if v := os.Getenv("NIUNIU_OCR_ENDPOINT"); v != "" {
		cfg.OCR.Endpoint = v
	}
	if v := os.Getenv("NIUNIU_OCR_MODEL"); v != "" {
		cfg.OCR.Model = v
	}
}

func (c *Config) validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("dataDir 不能为空")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d", c.Server.Port)
	}
	return nil
}

// AbsDataDir 返回数据目录的绝对路径。
func (c *Config) AbsDataDir() (string, error) {
	return filepath.Abs(c.DataDir)
}
