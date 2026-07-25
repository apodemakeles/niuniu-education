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

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Config 是应用运行配置，对应 data/config.yaml。
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	DataDir       string              `yaml:"dataDir"`
	Database      DBConfig            `yaml:"database"`
	OCR           OCRConfig           `yaml:"ocr"`
	LLM           LLMConfig           `yaml:"llm"`
	Pronunciation PronunciationConfig `yaml:"pronunciation"`
	Debug         DebugConfig         `yaml:"debug"`
	OpenBrowser   bool                `yaml:"openBrowser"`

	// ConfigFile 是配置文件自身的绝对路径，运行时填充，不写回 yaml。
	ConfigFile string `yaml:"-"`
}

// DebugConfig 本地开发/联调开关。正式给孩子用时应保持关闭。
type DebugConfig struct {
	// Enabled 开启后放宽部分学生端限制，便于家长/开发者快速走通流程。
	// 当前效果：延伸阅读不再要求停留满 minSeconds，可立即完成。
	Enabled bool `yaml:"enabled"`
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

// LLMConfig 文本大模型配置（学生端延伸阅读短文生成）。
// 默认走 DeepSeek 官方 OpenAI 兼容接口（deepseek-v4-flash，响应快）。
// APIKey 必须在 data/config.yaml 的 llm.apiKey 中填写。
type LLMConfig struct {
	Provider string `yaml:"provider"` // mock / siliconflow；为空按 siliconflow 处理
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	Model    string `yaml:"model"`
}

// PronunciationConfig 控制单词发音来源、地区与上游保护策略。
// providers 按优先级依次查询；未来的 TTS provider 也实现同一 PronunciationProvider 接口。
type PronunciationConfig struct {
	Locale             string   `yaml:"locale"`
	Providers          []string `yaml:"providers"`
	RequestIntervalMS  int      `yaml:"requestIntervalMs"`
	MaxRetries         int      `yaml:"maxRetries"`
	NegativeCacheHours int      `yaml:"negativeCacheHours"`
	// CorpusDir 为本地离线真人发音库根目录（其下应含 cambridge/、tfd/ 子目录）。
	// 配置后 providers 中的 cambridge/tfd 会按 {word}.mp3 查找本地音频，命中即用、零网络依赖。
	// 为空时这两个 provider 始终返回未找到，自动降级到后续在线来源。
	CorpusDir string `yaml:"corpusDir"`
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
		LLM: LLMConfig{
			Provider: "siliconflow",
			// DeepSeek 官方 OpenAI 兼容接口。
			// model 用 deepseek-v4-flash（deepseek-chat 已被官方下线，调用会返回 invalid_request_error）。
			// APIKey 必须在 data/config.yaml 的 llm.apiKey 中填写。
			Endpoint: "https://api.deepseek.com",
			Model:    "deepseek-v4-flash",
		},
		Pronunciation: PronunciationConfig{
			Locale:             "en-GB",
			// 离线真人库优先（命中即用、音质好、零网络），在线来源兜底。
			Providers:          []string{"cambridge", "tfd", "free_dictionary", "wiktionary"},
			RequestIntervalMS:  800,
			MaxRetries:         3,
			NegativeCacheHours: 24,
		},
		Debug: DebugConfig{
			Enabled: false,
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
	if v := os.Getenv("NIUNIU_LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("NIUNIU_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("NIUNIU_LLM_ENDPOINT"); v != "" {
		cfg.LLM.Endpoint = v
	}
	if v := os.Getenv("NIUNIU_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("NIUNIU_PRONUNCIATION_LOCALE"); v != "" {
		cfg.Pronunciation.Locale = v
	}
	if v := os.Getenv("NIUNIU_PRONUNCIATION_PROVIDERS"); v != "" {
		cfg.Pronunciation.Providers = splitCSV(v)
	}
	if v := os.Getenv("NIUNIU_PRONUNCIATION_CORPUS_DIR"); v != "" {
		cfg.Pronunciation.CorpusDir = v
	}
	if v := os.Getenv("NIUNIU_DEBUG"); v != "" {
		cfg.Debug.Enabled = envTruthy(v)
	}

	// LLM 默认走 DeepSeek 官方接口，与 OCR（硅基流动）独立配置。
	// APIKey 必须在 data/config.yaml 的 llm.apiKey 或环境变量 NIUNIU_LLM_API_KEY 中填写。
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = "siliconflow"
	}
	if cfg.LLM.Endpoint == "" {
		cfg.LLM.Endpoint = "https://api.deepseek.com"
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "deepseek-v4-flash"
	}
	if cfg.Pronunciation.Locale == "" {
		cfg.Pronunciation.Locale = "en-GB"
	}
	if len(cfg.Pronunciation.Providers) == 0 {
		cfg.Pronunciation.Providers = []string{"cambridge", "tfd", "free_dictionary", "wiktionary"}
	}
	if cfg.Pronunciation.RequestIntervalMS <= 0 {
		cfg.Pronunciation.RequestIntervalMS = 800
	}
	if cfg.Pronunciation.MaxRetries < 0 {
		cfg.Pronunciation.MaxRetries = 0
	}
	if cfg.Pronunciation.NegativeCacheHours <= 0 {
		cfg.Pronunciation.NegativeCacheHours = 24
	}
}

// LLMProviderName 暴露给 main 选择 provider 类型。
func (c *Config) LLMProviderName() string {
	if c.LLM.Provider == "mock" {
		return "mock"
	}
	return "siliconflow"
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
