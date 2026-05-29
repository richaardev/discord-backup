package config

import (
	"log"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Token        string `env:"DISCORD_TOKEN,required"`
	DatabasePath string `env:"DATABASE_PATH" envDefault:"./data/backups.db"`
}

func Must() Config {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		log.Fatal("failed to parse config", "err", err)
	}
	return cfg
}
