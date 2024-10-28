package main

import (
	"context"
	"net/http"
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/bufbuild/protovalidate-go"
	"github.com/tierklinik-dobersberg/apis/gen/go/tkd/comment/v1/commentv1connect"
	"github.com/tierklinik-dobersberg/apis/pkg/auth"
	"github.com/tierklinik-dobersberg/apis/pkg/cors"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery/consuldiscover"
	"github.com/tierklinik-dobersberg/apis/pkg/discovery/wellknown"
	"github.com/tierklinik-dobersberg/apis/pkg/log"
	"github.com/tierklinik-dobersberg/apis/pkg/server"
	"github.com/tierklinik-dobersberg/apis/pkg/validator"
	"github.com/tierklinik-dobersberg/comment-service/internal/config"
	"github.com/tierklinik-dobersberg/comment-service/internal/service"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func main() {
	ctx := context.Background()

	logger := log.L(ctx)

	var cfgFilePath string
	if len(os.Args) > 1 {
		cfgFilePath = os.Args[1]
	}

	cfg, err := config.LoadConfig(ctx, cfgFilePath)
	if err != nil {
		logger.Fatalf("failed to load configuration: %s", err)
	}
	logger.Infof("configuration loaded successfully")

	providers, err := config.NewProviders(ctx, *cfg)
	if err != nil {
		logger.Fatalf("failed to prepare providers: %s", err)
	}
	logger.Infof("application providers prepared successfully")

	protoValidator, err := protovalidate.New()
	if err != nil {
		logger.Fatalf("failed to prepare protovalidator: %s", err)
	}

	authInterceptor := auth.NewAuthAnnotationInterceptor(
		protoregistry.GlobalFiles,
		auth.NewIDMRoleResolver(providers.Roles),
		auth.RemoteHeaderExtractor)

	interceptors := connect.WithInterceptors(
		log.NewLoggingInterceptor(),
		authInterceptor,
		validator.NewInterceptor(protoValidator),
	)

	corsConfig := cors.Config{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowCredentials: true,
	}

	// Prepare our servemux and add handlers.
	serveMux := http.NewServeMux()

	// create a new CallService and add it to the mux.
	svc := service.New(providers)

	path, handler := commentv1connect.NewCommentServiceHandler(svc, interceptors)
	serveMux.Handle(path, handler)

	// Register at service catalog
	catalog, err := consuldiscover.NewFromEnv()
	if err != nil {
		logger.Fatalf("failed to get service catalog client: %s", err)
	}

	if err := discovery.Register(ctx, catalog, &discovery.ServiceInstance{
		Name:    wellknown.CommentV1ServiceScope,
		Address: cfg.PublicListenAddress,
	}); err != nil {
		logger.Errorf("failed to register comment service at service catalog: %s", err)
	}

	// Create the server
	srv := server.Create(cfg.PublicListenAddress, cors.Wrap(corsConfig, serveMux))

	logger.Infof("HTTP/2 server (h2c) prepared successfully, startin to listen ...")

	if err := server.Serve(ctx, srv); err != nil {
		logger.Fatalf("failed to serve: %s", err)
	}
}
