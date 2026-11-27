package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application, infrastructure, and AI provider settings.
type Config struct {
	App      AppConfig
	Redis    RedisConfig
	Temporal TemporalConfig
	Storage  StorageConfig
	AI       AIConfig
	FFmpeg   FFmpegConfig
	Limits   LimitsConfig
}

type AppConfig struct {
	Env      string `mapstructure:"APP_ENV"`
	Port     int    `mapstructure:"APP_PORT"`
	Secret   string `mapstructure:"APP_SECRET"`
	LogLevel string `mapstructure:"LOG_LEVEL"`
}

type RedisConfig struct {
	URL string `mapstructure:"REDIS_URL"`
}

type TemporalConfig struct {
	Host      string `mapstructure:"TEMPORAL_HOST"`
	Namespace string `mapstructure:"TEMPORAL_NAMESPACE"`
	TaskQueue string `mapstructure:"TEMPORAL_TASK_QUEUE"`
}

type StorageConfig struct {
	Provider      string `mapstructure:"STORAGE_PROVIDER"`
	LocalDir      string `mapstructure:"LOCAL_STORAGE_DIR"`
	TTLHours      int    `mapstructure:"STORAGE_TTL_HOURS"`
	S3Endpoint    string `mapstructure:"S3_ENDPOINT"`
	S3AccessKey   string `mapstructure:"S3_ACCESS_KEY"`
	S3SecretKey   string `mapstructure:"S3_SECRET_KEY"`
	S3Bucket      string `mapstructure:"S3_BUCKET"`
	S3Region      string `mapstructure:"S3_REGION"`
	S3UseSSL      bool   `mapstructure:"S3_USE_SSL"`
}

type AIConfig struct {
	// ASR
	ASRProvider         string `mapstructure:"ASR_PROVIDER"`
	ASRFallbackProvider string `mapstructure:"ASR_FALLBACK_PROVIDER"`
	ASRModel            string `mapstructure:"ASR_MODEL"`
	GroqAPIKey          string `mapstructure:"GROQ_API_KEY"`
	GroqASRModel        string `mapstructure:"GROQ_ASR_MODEL"`
	OpenAIAPIKey        string `mapstructure:"OPENAI_API_KEY"`
	OpenAIASRModel      string `mapstructure:"OPENAI_ASR_MODEL"`
	DeepgramAPIKey      string `mapstructure:"DEEPGRAM_API_KEY"`
	DeepgramModel       string `mapstructure:"DEEPGRAM_MODEL"`

	// LLM
	LLMProvider         string `mapstructure:"LLM_PROVIDER"`
	LLMFallbackProvider string `mapstructure:"LLM_FALLBACK_PROVIDER"`
	LLMModel            string `mapstructure:"LLM_MODEL"`
	GroqLLMModel        string `mapstructure:"GROQ_LLM_MODEL"`
	OpenAILLMModel      string `mapstructure:"OPENAI_LLM_MODEL"`
	GeminiAPIKey        string `mapstructure:"GEMINI_API_KEY"`
	GeminiLLMModel      string `mapstructure:"GEMINI_LLM_MODEL"`
	AnthropicAPIKey     string `mapstructure:"ANTHROPIC_API_KEY"`
	AnthropicLLMModel   string `mapstructure:"ANTHROPIC_LLM_MODEL"`
	DeepSeekAPIKey      string `mapstructure:"DEEPSEEK_API_KEY"`
	DeepSeekLLMModel    string `mapstructure:"DEEPSEEK_LLM_MODEL"`

	// ImageGen
	ImageGenProvider         string `mapstructure:"IMAGEGEN_PROVIDER"`
	ImageGenFallbackProvider string `mapstructure:"IMAGEGEN_FALLBACK_PROVIDER"`
	ImageGenModel            string `mapstructure:"IMAGEGEN_MODEL"`
	OpenAIImageGenModel      string `mapstructure:"OPENAI_IMAGEGEN_MODEL"`
	ReplicateAPIKey          string `mapstructure:"REPLICATE_API_KEY"`
	ReplicateImageGenModel   string `mapstructure:"REPLICATE_IMAGEGEN_MODEL"`
	FalAPIKey                string `mapstructure:"FAL_API_KEY"`
	FalImageGenModel         string `mapstructure:"FAL_IMAGEGEN_MODEL"`

	// Vision
	VisionProvider         string `mapstructure:"VISION_PROVIDER"`
	VisionFallbackProvider string `mapstructure:"VISION_FALLBACK_PROVIDER"`
	VisionModel            string `mapstructure:"VISION_MODEL"`
	OpenAIVisionModel      string `mapstructure:"OPENAI_VISION_MODEL"`
	GeminiVisionModel      string `mapstructure:"GEMINI_VISION_MODEL"`

	// VideoGen
	VideoGenProvider         string `mapstructure:"VIDEOGEN_PROVIDER"`
	VideoGenFallbackProvider string `mapstructure:"VIDEOGEN_FALLBACK_PROVIDER"`
	VideoGenModel            string `mapstructure:"VIDEOGEN_MODEL"`
	ReplicateVideoGenModel   string `mapstructure:"REPLICATE_VIDEOGEN_MODEL"`
	FalVideoGenModel         string `mapstructure:"FAL_VIDEOGEN_MODEL"`
}

type FFmpegConfig struct {
	FFmpegPath  string `mapstructure:"FFMPEG_PATH"`
	FFprobePath string `mapstructure:"FFPROBE_PATH"`
}

type LimitsConfig struct {
	MaxVideoDurationSec int `mapstructure:"MAX_VIDEO_DURATION_SEC"`
	MaxFileSizeMB       int `mapstructure:"MAX_FILE_SIZE_MB"`
	MaxImagesPerScene   int `mapstructure:"MAX_IMAGES_PER_SCENE"`
	MaxRegenAttempts    int `mapstructure:"MAX_REGEN_ATTEMPTS"`
	MaxConcurrentJobs   int `mapstructure:"MAX_CONCURRENT_JOBS"`
}

// Load reads configuration from the given .env file path (optional) and environment variables.
func Load(envFile string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if envFile != "" {
		v.SetConfigFile(envFile)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %s: %w", envFile, err)
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{}
	if err := v.Unmarshal(&cfg.App); err != nil {
		return nil, fmt.Errorf("unmarshal app config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Redis); err != nil {
		return nil, fmt.Errorf("unmarshal redis config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Temporal); err != nil {
		return nil, fmt.Errorf("unmarshal temporal config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Storage); err != nil {
		return nil, fmt.Errorf("unmarshal storage config: %w", err)
	}
	if err := v.Unmarshal(&cfg.AI); err != nil {
		return nil, fmt.Errorf("unmarshal ai config: %w", err)
	}
	if err := v.Unmarshal(&cfg.FFmpeg); err != nil {
		return nil, fmt.Errorf("unmarshal ffmpeg config: %w", err)
	}
	if err := v.Unmarshal(&cfg.Limits); err != nil {
		return nil, fmt.Errorf("unmarshal limits config: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("LOG_LEVEL", "info")

	v.SetDefault("TEMPORAL_HOST", "localhost:7233")
	v.SetDefault("TEMPORAL_NAMESPACE", "default")
	v.SetDefault("TEMPORAL_TASK_QUEUE", "video-processing")

	v.SetDefault("STORAGE_PROVIDER", "local")
	v.SetDefault("LOCAL_STORAGE_DIR", "./tmp/storage")
	v.SetDefault("STORAGE_TTL_HOURS", 24)
	v.SetDefault("S3_REGION", "us-east-1")
	v.SetDefault("S3_BUCKET", "reel-assets")

	v.SetDefault("ASR_PROVIDER", "groq")
	v.SetDefault("LLM_PROVIDER", "groq")
	v.SetDefault("IMAGEGEN_PROVIDER", "replicate")
	v.SetDefault("VISION_PROVIDER", "openai")
	v.SetDefault("VIDEOGEN_PROVIDER", "replicate")

	v.SetDefault("FFMPEG_PATH", "ffmpeg")
	v.SetDefault("FFPROBE_PATH", "ffprobe")

	v.SetDefault("MAX_VIDEO_DURATION_SEC", 600)
	v.SetDefault("MAX_FILE_SIZE_MB", 500)
	v.SetDefault("MAX_IMAGES_PER_SCENE", 3)
	v.SetDefault("MAX_REGEN_ATTEMPTS", 5)
	v.SetDefault("MAX_CONCURRENT_JOBS", 10)
}
