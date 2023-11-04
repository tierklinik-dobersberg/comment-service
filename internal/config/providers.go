package config

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/idm/v1/idmv1connect"
	"github.com/tierklinik-dobersberg/comment-service/internal/repo"
)

type Providers struct {
	Users  idmv1connect.UserServiceClient
	Roles  idmv1connect.RoleServiceClient
	Notify idmv1connect.NotifyServiceClient

	Repository *repo.Repository

	Config Config
}

func NewProviders(ctx context.Context, cfg Config) (*Providers, error) {
	httpClient := http.DefaultClient

	repo, err := repo.NewRepository(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	p := &Providers{
		Users:      idmv1connect.NewUserServiceClient(httpClient, cfg.IdmURL),
		Roles:      idmv1connect.NewRoleServiceClient(httpClient, cfg.IdmURL),
		Notify:     idmv1connect.NewNotifyServiceClient(httpClient, cfg.IdmURL),
		Repository: repo,
		Config:     cfg,
	}

	return p, nil
}
