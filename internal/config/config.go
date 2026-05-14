package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config хранит настройки, нужные для запуска локального приложения.
type Config struct {
	Addr            string
	DataDir         string
	SQLitePath      string
	ShutdownTimeout time.Duration
}

// envConfig описывает переменные окружения, которые читает cleanenv.
type envConfig struct {
	Addr            string `env:"PKB_ADDR" env-default:":8080"`
	DataDir         string `env:"PKB_DATA_DIR" env-default:"data"`
	SQLitePath      string `env:"PKB_SQLITE_PATH"`
	ShutdownTimeout string `env:"PKB_SHUTDOWN_TIMEOUT" env-default:"5s"`
}

// Load читает настройки из переменных окружения и заполняет значения по умолчанию.
func Load() (Config, error) {
	var env envConfig
	if err := cleanenv.ReadEnv(&env); err != nil {
		return Config{}, fmt.Errorf("read env config: %w", err)
	}

	cfg := Config{
		Addr:    env.Addr,
		DataDir: env.DataDir,
	}

	cfg.SQLitePath = env.SQLitePath
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = filepath.Join(cfg.DataDir, "pkb.sqlite3")
	}

	shutdownTimeout, err := time.ParseDuration(env.ShutdownTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse PKB_SHUTDOWN_TIMEOUT: %w", err)
	}
	if shutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("PKB_SHUTDOWN_TIMEOUT must be positive")
	}
	cfg.ShutdownTimeout = shutdownTimeout

	return cfg, nil
}
