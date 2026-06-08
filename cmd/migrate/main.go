package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ficct-boutique/backend-go/internal/config"
	"github.com/ficct-boutique/backend-go/internal/database"
	"github.com/ficct-boutique/backend-go/internal/observability"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	observability.InitLogger(cfg.LogLevel, cfg.AppEnv)

	direction := database.DirectionUp
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "up":
			direction = database.DirectionUp
		case "down":
			direction = database.DirectionDown
		default:
			fmt.Fprintln(os.Stderr, "usage: migrate [up|down]")
			os.Exit(2)
		}
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect database")
	}
	defer pool.Close()

	log.Info().Str("dir", cfg.MigrationsDir).Str("direction", string(direction)).Msg("running migrations")
	if err := database.RunMigrations(ctx, pool, cfg.MigrationsDir, direction); err != nil {
		log.Fatal().Err(err).Msg("migrate")
	}
	log.Info().Msg("migrations complete")
}
