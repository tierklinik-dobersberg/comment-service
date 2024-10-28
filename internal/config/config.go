package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	IdmURL              string   `env:"IDM_URL" json:"idmURL"`
	Database            string   `env:"DATABASE" json:"database"`
	AllowedOrigins      []string `env:"ALLOWED_ORIGINS" json:"allowedOrigins"`
	PublicListenAddress string   `env:"PUBLIC_LISTEN" json:"publicListen"`
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
	var cfg Config

	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file at path %q: %w", path, err)
		}

		switch filepath.Ext(path) {
		case ".yaml", ".yml":
			content, err = yaml.YAMLToJSON(content)
			if err != nil {
				return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}

			fallthrough
		case ".json":
			dec := json.NewDecoder(bytes.NewReader(content))
			dec.DisallowUnknownFields()

			if err := dec.Decode(&cfg); err != nil {
				return nil, fmt.Errorf("failed to decode JSON: %w", err)
			}
		}
	}

	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration from environment: %w", err)
	}

	if cfg.PublicListenAddress == "" {
		cfg.PublicListenAddress = ":8080"
	}

	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}

	if cfg.IdmURL == "" {
		return nil, fmt.Errorf("missing idmUrl config setting")
	}

	if _, err := url.Parse(cfg.IdmURL); err != nil {
		return nil, fmt.Errorf("invalid IDM_URL: %w", err)
	}

	if cfg.Database == "" {
		return nil, fmt.Errorf("missing database")
	}

	if _, err := url.Parse(cfg.Database); err != nil {
		return nil, fmt.Errorf("invalid DATABASE: %w", err)
	}

	return &cfg, nil
}
