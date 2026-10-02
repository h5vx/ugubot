// Package history owns the chat archive: it stores events coming from the bus
// and answers queries about chats, messages and dates.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nats.go/micro"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/pg"
)

const (
	consumerName   = "history"
	requestTimeout = 15 * time.Second
	maxLastN       = 1000
)

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Service struct {
	store *Store
	nc    *nats.Conn
	js    jetstream.JetStream
	log   *slog.Logger
}

func Run(ctx context.Context, pool *pgxpool.Pool, nc *nats.Conn) error {
	if err := pg.Migrate(ctx, pool, pg.MustSub(migrations, "migrations"), "goose_version_chat"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := bus.EnsureStreams(ctx, js); err != nil {
		return err
	}

	s := &Service{store: NewStore(pool), nc: nc, js: js, log: slog.With("service", "history")}

	if err := s.addEndpoints(); err != nil {
		return err
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, bus.StreamEvents, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxAckPending: 1, // keep insertion order equal to event order
	})
	if err != nil {
		return fmt.Errorf("consumer: %w", err)
	}

	cc, err := consumer.Consume(s.handleEvent)
	if err != nil {
		return err
	}
	defer cc.Stop()

	s.log.Info("history service started")
	<-ctx.Done()
	return nil
}

func (s *Service) handleEvent(m jetstream.Msg) {
	var ev bus.Event
	if err := json.Unmarshal(m.Data(), &ev); err != nil {
		s.log.Error("bad event, dropping", "subject", m.Subject(), "err", err)
		_ = m.Term()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	msg, err := s.store.SaveEvent(ctx, &ev)
	if err != nil {
		s.log.Error("store event", "event", ev.ID, "err", err)
		_ = m.NakWithDelay(2 * time.Second)
		return
	}

	// Message id as dedup key: a redelivered event is not announced twice.
	if err := bus.Publish(ctx, s.js, bus.StoredSubject(msg.ChatID), fmt.Sprintf("msg-%d", msg.ID), msg); err != nil {
		s.log.Error("publish stored message", "id", msg.ID, "err", err)
		_ = m.NakWithDelay(2 * time.Second)
		return
	}

	_ = m.Ack()
}

func (s *Service) addEndpoints() error {
	svc, err := micro.AddService(s.nc, micro.Config{
		Name:        "history",
		Version:     "1.0.0",
		Description: "ugubot chat archive",
	})
	if err != nil {
		return err
	}

	type empty struct{}

	endpoints := map[string]micro.Handler{
		bus.SubjectHistoryChats: bus.JSONHandler(requestTimeout, func(ctx context.Context, _ empty) ([]bus.Chat, error) {
			return s.store.Chats(ctx)
		}),
		bus.SubjectHistoryChat: bus.JSONHandler(requestTimeout, func(ctx context.Context, r bus.ChatRequest) (*bus.Chat, error) {
			return s.store.Chat(ctx, r.ChatID)
		}),
		bus.SubjectHistoryMessages: bus.JSONHandler(requestTimeout, func(ctx context.Context, r bus.MessagesRequest) ([]bus.Message, error) {
			return s.store.Messages(ctx, r.ChatID, r.Date, r.Timezone)
		}),
		bus.SubjectHistoryDates: bus.JSONHandler(requestTimeout, func(ctx context.Context, r bus.DatesRequest) ([]string, error) {
			start := time.Now()
			dates, err := s.store.Dates(ctx, r.ChatID, r.Timezone)
			s.log.Debug("dates", "chat", r.ChatID, "tz", r.Timezone, "days", len(dates), "took", time.Since(start))
			return dates, err
		}),
		bus.SubjectHistoryLastN: bus.JSONHandler(requestTimeout, func(ctx context.Context, r bus.LastNRequest) ([]bus.Message, error) {
			n := min(max(r.N, 1), maxLastN)
			return s.store.LastN(ctx, r.ChatID, n, r.Kinds)
		}),
		bus.SubjectHistoryNickColors: bus.JSONHandler(requestTimeout, func(ctx context.Context, _ empty) ([]bus.NickColor, error) {
			return s.store.NickColors(ctx)
		}),
		bus.SubjectHistorySetNickColor: bus.JSONHandler(requestTimeout, func(ctx context.Context, r bus.NickColor) (string, error) {
			if r.Nick == "" {
				return "", fmt.Errorf("nick is required")
			}
			if r.Color != "" && !colorRe.MatchString(r.Color) {
				return "", fmt.Errorf("invalid color %q: expected #rrggbb", r.Color)
			}
			return "OK", s.store.SetNickColor(ctx, r.Nick, r.Color)
		}),
	}

	for subject, h := range endpoints {
		name := strings.ReplaceAll(strings.TrimPrefix(subject, "ugubot.history."), ".", "_")
		if err := svc.AddEndpoint(name, h, micro.WithEndpointSubject(subject)); err != nil {
			return fmt.Errorf("endpoint %s: %w", subject, err)
		}
	}
	return nil
}
