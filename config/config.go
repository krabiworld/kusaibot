package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/rs/zerolog/log"
)

type Config struct {
	DiscordToken string `env:"DISCORD_TOKEN,required"`
	DiscordGuild string `env:"DISCORD_GUILD,required"`
	DiscordBot   string `env:"DISCORD_BOT,required"`
	GRPCAddr     string `env:"GRPC_ADDR,required"`
}

func Parse() Config {
	var cfg Config
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot parse config")
	}

	return cfg
}
