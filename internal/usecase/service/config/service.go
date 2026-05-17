package config

type RepositoryConfig interface {
}

// Service позволяет пользователю конфигурировать приложение на ходу
type Service struct {
	repositories RepositoryConfig
	cfg          *Config
}

func NewService(cfg *Config, repositories RepositoryConfig) (*Service, error) {
	return &Service{
		cfg:          cfg,
		repositories: repositories,
	}, nil
}
