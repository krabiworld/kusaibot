package main

import (
	"context"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/krabiworld/kusaibot/config"
	"github.com/krabiworld/kusaibot/proto"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stdout})

	log.Info().Msg("Starting...")

	cfg := config.Parse()

	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot connect to gRPC server")
	}
	defer func(conn *grpc.ClientConn) {
		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("Cannot close gRPC connection")
		}
	}(conn)
	rpc := proto.NewTextChainClient(conn)

	log.Info().Msg("Connected to gRPC server.")

	s, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot create Discord session")
	}

	var (
		emojiMap = make(map[string]string)
		mu       sync.RWMutex
		re       = regexp.MustCompile(`:([a-zA-Z0-9_]+):`)
	)

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Info().Msgf("Logged in as: %s#%s", s.State.User.Username, s.State.User.Discriminator)
	})
	s.AddHandler(func(s *discordgo.Session, r *discordgo.MessageCreate) {
		if r.Author.Bot || r.GuildID != cfg.DiscordGuild {
			return
		}

		log.Info().Msg("Message from Discord: " + r.Content)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = rpc.Train(ctx, &proto.TrainRequest{
			Sequences: []string{"\x02 " + r.Content + " \x03"},
		})
		if err != nil {
			log.Error().Err(err).Msg("Cannot train")
		}

		mentioned := false
		for _, u := range r.Mentions {
			if u.ID == cfg.DiscordBot {
				mentioned = true
				break
			}
		}

		if strings.HasPrefix(r.Content, "!k") ||
			mentioned ||
			rand.Intn(50) == 0 {
			tokens, err := rpc.GenerateTokens(ctx, &proto.GenerateTokensRequest{
				Context: "\x02",
			})
			if err != nil {
				log.Error().Err(err).Msg("Cannot generate tokens")
			}

			msg := strings.Trim(tokens.Text, "\x02\x03 ")

			mu.RLock()
			msg = re.ReplaceAllStringFunc(msg, func(match string) string {
				name := strings.Trim(match, ":")
				if replacement, ok := emojiMap[name]; ok {
					return replacement
				}
				return match
			})
			mu.RUnlock()

			_, err = s.ChannelMessageSendReply(r.ChannelID, msg, r.Reference())
			if err != nil {
				log.Error().Err(err).Msg("Cannot send message")
			}
		}
	})

	err = s.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot open Discord session")
	}
	defer func(s *discordgo.Session) {
		if err := s.Close(); err != nil {
			log.Error().Err(err).Msg("Cannot close Discord session")
		}
	}(s)

	guild, err := s.Guild(cfg.DiscordGuild)
	if err != nil {
		log.Error().Err(err).Msg("Cannot fetch Discord guild")
	}

	mu.Lock()
	for _, e := range guild.Emojis {
		emojiMap[e.Name] = e.MessageFormat()
	}
	log.Info().Int("count", len(emojiMap)).Msg("Emojis loaded")
	mu.Unlock()

	log.Info().Msg("Bot is now running.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Info().Msg("Gracefully shutting down.")
}
