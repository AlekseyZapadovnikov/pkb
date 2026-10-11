package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds the settings required to run the local application.
type Config struct {
	Addr               string        `env:"PKB_ADDR" env-default:":8080"`
	DataDir            string        `env:"PKB_DATA_DIR" env-default:"data"`
	SQLitePath         string        `env:"PKB_SQLITE_PATH"`
	ShutdownTimeout    time.Duration `env:"PKB_SHUTDOWN_TIMEOUT" env-default:"5s"`
	MemoryPaths        MemoryPaths
	ClassifierProvider ProviderConfig
}

type MemoryPaths struct {
	Vector string `env:"PKB_VECTOR_PATH" validate:"required"`          // Path to vector.so.
	Memory string `env:"PKB_MEMORY_PATH" validate:"required"`          // Path to memory.so.
	Model  string `env:"PKB_EMBEDDING_MODEL_PATH" validate:"required"` // Path to the .gguf embedding model.
}

type ProviderConfig struct {
	Name    string `env:"PKB_MODEL_PROVIDER_NAME"`
	Kind    string `env:"PKB_MODEL_PROVIDER_KIND"` // openai_compatible, gemini, etc
	BaseURL string `env:"PKB_MODEL_BASE_URL" env-default:"https://api.openai.com/v1"`
	APIKey  string `env:"PKB_MODEL_API_KEY"`
	Model   string `env:"PKB_MODEL_NAME"`
}

// Load reads settings from environment variables and applies default values.
func Load() (Config, error) {
	var cfg Config

	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		return Config{}, fmt.Errorf("read .env config: %w", err)
	}

	if cfg.SQLitePath == "" {
		cfg.SQLitePath = filepath.Join(cfg.DataDir, "pkb.sqlite3")
	}

	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("PKB_SHUTDOWN_TIMEOUT must be positive")
	}

	return cfg, nil
}
