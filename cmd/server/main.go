package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ficct-boutique/backend-go/graph"
	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/config"
	"github.com/ficct-boutique/backend-go/internal/database"
	"github.com/ficct-boutique/backend-go/internal/middleware"
	"github.com/ficct-boutique/backend-go/internal/observability"
	"github.com/ficct-boutique/backend-go/internal/repository"
	"github.com/ficct-boutique/backend-go/internal/service"
	"github.com/ficct-boutique/backend-go/internal/staticassets"
	"github.com/ficct-boutique/backend-go/internal/webhook"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	graphqlgo "github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/rs/zerolog/log"
)

//go:embed schema.graphqls.embed
var embeddedSchema string

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	observability.InitLogger(cfg.LogLevel, cfg.AppEnv)

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect database")
	}
	defer pool.Close()

	keys, err := auth.LoadKeyPair(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath, cfg.JWTKeyID)
	if err != nil {
		log.Fatal().Err(err).Msg("load keys")
	}
	issuer, err := auth.NewIssuer(keys, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTAccessTTL)
	if err != nil {
		log.Fatal().Err(err).Msg("create token issuer")
	}
	verifier := auth.NewVerifier(keys, cfg.JWTIssuer, "ficct-angular")

	userRepo := repository.NewUserRepo(pool)
	catalogRepo := repository.NewCatalogRepo(pool)
	branchRepo := repository.NewBranchRepo(pool)
	invRepo := repository.NewInventoryRepo(pool)
	salesRepo := repository.NewSalesRepo(pool)
	orderRepo := repository.NewOrderRepo(pool)
	reportsRepo := repository.NewReportsRepo(pool)
	outboxRepo := repository.NewOutboxRepo(pool)

	authSvc := service.NewAuthService(userRepo, issuer)
	catalogSvc := service.NewCatalogService(catalogRepo, branchRepo, invRepo)
	reportsSvc := service.NewReportsService(reportsRepo)
	salesSvc := service.NewSalesService(service.SalesServiceDeps{
		Sales:            salesRepo,
		Inventory:        invRepo,
		Orders:           orderRepo,
		Catalog:          catalogRepo,
		Outbox:           outboxRepo,
		WebhookTargetURL: cfg.WebhookInvoiceURL,
	})

	if cfg.WebhookInvoiceURL != "" && cfg.WebhookHMACSecret != "" {
		dispatcher := webhook.NewDispatcher(outboxRepo, cfg.WebhookHMACSecret, cfg.WebhookDispatchInterval, cfg.WebhookMaxRetries)
		go dispatcher.Run(rootCtx)
	} else {
		log.Warn().Msg("webhook dispatcher not started (URL or secret missing)")
	}

	rootResolver := &graph.Resolver{
		AuthSvc:     authSvc,
		CatalogSvc:  catalogSvc,
		SalesSvc:    salesSvc,
		ReportsSvc:  reportsSvc,
		UserRepo:    userRepo,
		CatalogRepo: catalogRepo,
		BranchRepo:  branchRepo,
		InvRepo:     invRepo,
		SalesRepo:   salesRepo,
		OrderRepo:   orderRepo,
	}

	gqlSchema, err := graphqlgo.ParseSchema(embeddedSchema, rootResolver,
		graphqlgo.UseFieldResolvers(),
		graphqlgo.MaxParallelism(20),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("parse schema")
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(requestLogger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	r.Use(rateLimiter.Middleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Seeded SVG product placeholders. Real catalog images are uploaded by
	// admins via the documents service; this route only powers demo seed data.
	r.Mount("/static/products/", staticassets.Handler())

	r.With(middleware.OptionalAuth(verifier)).Handle("/graphql", &relay.Handler{Schema: gqlSchema})
	r.Get("/playground", playgroundHandler())

	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.AppPort).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info().Msg("shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	rateLimiter.Stop()
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("dur", time.Since(start)).
			Msg("request")
	})
}

func playgroundHandler() http.HandlerFunc {
	const html = `<!DOCTYPE html>
<html>
  <head>
    <title>FICCT Boutique GraphQL Playground</title>
    <link rel="stylesheet" href="https://unpkg.com/graphiql/graphiql.min.css" />
  </head>
  <body style="margin:0">
    <div id="graphiql" style="height:100vh"></div>
    <script src="https://unpkg.com/react@17/umd/react.production.min.js"></script>
    <script src="https://unpkg.com/react-dom@17/umd/react-dom.production.min.js"></script>
    <script src="https://unpkg.com/graphiql/graphiql.min.js"></script>
    <script>
      const fetcher = GraphiQL.createFetcher({ url: '/graphql' });
      ReactDOM.render(React.createElement(GraphiQL, { fetcher }), document.getElementById('graphiql'));
    </script>
  </body>
</html>`
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}
}
