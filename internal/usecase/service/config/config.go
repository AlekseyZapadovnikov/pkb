package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config хранит настройки, нужные для запуска локального приложения.
type Config struct {
	Addr            string        `env:"PKB_ADDR" env-default:":8080"`
	DataDir         string        `env:"PKB_DATA_DIR" env-default:"data"`
	SQLitePath      string        `env:"PKB_SQLITE_PATH"`
	ShutdownTimeout time.Duration `env:"PKB_SHUTDOWN_TIMEOUT" env-default:"5s"`
}

type ProviderConfig struct {
	Name     string
	Kind     string // openai_compatible, gemini, etc
	BaseURL  string
	APIKey   string
	Model    string
	ProxyURL string
}

// Load читает настройки из переменных окружения и заполняет значения по умолчанию.
func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read env config: %w", err)
	}

	if cfg.SQLitePath == "" {
		cfg.SQLitePath = filepath.Join(cfg.DataDir, "pkb.sqlite3")
	}

	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("PKB_SHUTDOWN_TIMEOUT must be positive")
	}

	return cfg, nil
}
