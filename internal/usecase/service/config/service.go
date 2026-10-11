package config

type RepositoryConfig interface {
}

// Service allows the user to configure the application at runtime.
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
