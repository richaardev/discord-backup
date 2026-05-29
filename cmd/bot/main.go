package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	disbot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/gateway"
	"github.com/joho/godotenv"
	"github.com/richaardev/discord-backuper/internal/bot"
	"github.com/richaardev/discord-backuper/internal/config"
	"github.com/richaardev/discord-backuper/internal/database"
	"github.com/richaardev/discord-backuper/internal/database/sqlc"
	"github.com/richaardev/discord-backuper/internal/interactions"
)

func init() {
	slog.SetDefault(
		slog.New(
			log.NewWithOptions(os.Stdout, log.Options{
				TimeFormat:      time.StampMilli,
				ReportTimestamp: true,
			}),
		),
	)

	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}
}

func main() {
	slog.Info("Starting backup bot...")

	cfg := config.Must()

	db := database.MustOpen(cfg.DatabasePath)
	defer db.Close()

	queries := sqlc.New(db)

	client, err := bot.NewClient(
		cfg.Token,
		bot.WithDisgoOpts(
			disbot.WithGatewayConfigOpts(
				gateway.WithIntents(
					gateway.IntentsAll,
				),
			),
			disbot.WithCacheConfigOpts(
				cache.WithCaches(
					cache.FlagGuilds | cache.FlagRoles | cache.FlagChannels,
				),
			),
		),
	)
	if err != nil {
		slog.Error("Error creating Disgo client", slog.Any("err", err))
		return
	}

	client.AddEventListeners(bot.OnReady(client))
	client.InteractionManager.RegisterSlashCommands(interactions.SlashBackup(queries, db))

	slog.Info("Opening gateway...")
	err = client.OpenGateway(context.TODO())
	if err != nil {
		slog.Error("error while connecting to gateway", slog.Any("err", err))
		return
	}
	defer client.Close(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	slog.Info("Received signal", "signal", sig)
}
