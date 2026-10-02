// Package ai answers chat messages with an OpenAI-compatible model.
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/config"
	"github.com/h5vx/ugubot/internal/pg"
)

const (
	consumerName = "ai-worker"
	// Messages older than this (e.g. received while the worker was down) are ignored.
	maxMessageAge     = 10 * time.Minute
	completionTimeout = 3 * time.Minute
	historyForContext = 300
)

// Completer calls the model. Implemented by OpenAI and by fakes in tests.
type Completer interface {
	Complete(ctx context.Context, model string, messages []ChatMessage, maxTokens int, temperature *float64) (*Completion, error)
}

type Completion struct {
	Text  string
	Model string
	Usage bus.Usage
}

// Sender delivers answers to the chat.
type Sender interface {
	Send(ctx context.Context, cmd *bus.SendCommand) error
}

type Worker struct {
	cfg       config.OpenAI
	chain     []Middleware
	context   *Context
	store     Store
	completer Completer
	sender    Sender
	log       *slog.Logger

	mu     sync.Mutex
	queues map[int64]chan *Incoming
}

func NewWorker(cfg config.OpenAI, admins []string, store Store, completer Completer, sender Sender) *Worker {
	prices := Prices(cfg.Prices)
	ctxMW := &Context{
		Store:        store,
		Tokens:       NewTokenCounter(),
		BotNick:      cfg.UserNick,
		DefaultModel: cfg.Model,
		MaxTokens:    cfg.MaxTokens - cfg.TokensReservedForResponse,
	}

	return &Worker{
		cfg:     cfg,
		context: ctxMW,
		chain: []Middleware{
			&DropIfDisabled{Enabled: cfg.Enabled},
			StripText{},
			&DropIfNotAddressed{BotNick: cfg.UserNick},
			&ParseCommands{Prefix: cfg.CommandPrefix},
			&Blocklist{Store: store, Admins: admins},
			&SwitchModel{Command: cfg.ModelSecondaryCommand, Model: cfg.ModelSecondary},
			Temperature{},
			&UsageReport{Store: store, Prices: prices},
			&InlineUsage{Prices: prices},
			Help{},
			&UserPrompts{Prompts: cfg.Prompt},
			ctxMW,
		},
		store:     store,
		completer: completer,
		sender:    sender,
		log:       slog.With("service", "ai-worker"),
		queues:    map[int64]chan *Incoming{},
	}
}

func Run(ctx context.Context, cfg config.OpenAI, admins []string, pool *pgxpool.Pool, nc *nats.Conn) error {
	if err := pg.Migrate(ctx, pool, pg.MustSub(migrations, "migrations"), "goose_version_ai"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := bus.EnsureStreams(ctx, js); err != nil {
		return err
	}

	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := openai.NewClient(opts...)

	w := NewWorker(cfg, admins, NewStore(pool), &openAICompleter{client: &client}, &jsSender{js: js})

	// The consumer is created first so messages arriving while the context
	// loads are not missed.
	consumer, err := js.CreateOrUpdateConsumer(ctx, bus.StreamStored, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("consumer: %w", err)
	}

	if err := w.loadState(ctx, nc); err != nil {
		return err
	}
	cc, err := consumer.Consume(func(m jetstream.Msg) {
		var msg bus.Message
		if err := json.Unmarshal(m.Data(), &msg); err != nil {
			w.log.Error("bad stored message", "err", err)
			_ = m.Term()
			return
		}
		w.HandleStored(ctx, &msg)
		_ = m.Ack()
	})
	if err != nil {
		return err
	}
	defer cc.Stop()

	w.log.Info("ai worker started", "enabled", cfg.Enabled, "model", cfg.Model)
	<-ctx.Done()
	return nil
}

// loadState restores preludes and conversation contexts.
func (w *Worker) loadState(ctx context.Context, nc *nats.Conn) error {
	preludes, err := w.store.Preludes(ctx)
	if err != nil {
		return fmt.Errorf("load preludes: %w", err)
	}
	for chatID, text := range preludes {
		w.context.LoadPrelude(chatID, text)
	}

	// history may still be starting
	var chats []bus.Chat
	for attempt := 1; ; attempt++ {
		rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		chats, err = bus.Request[[]bus.Chat](rctx, nc, bus.SubjectHistoryChats, struct{}{})
		cancel()
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		w.log.Warn("waiting for history service", "err", err)
		time.Sleep(min(time.Duration(attempt)*time.Second, 10*time.Second))
	}

	for _, chat := range chats {
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		msgs, err := bus.Request[[]bus.Message](rctx, nc, bus.SubjectHistoryLastN, bus.LastNRequest{
			ChatID: chat.ID,
			N:      historyForContext,
			Kinds:  []bus.Kind{bus.KindUser, bus.KindForAI},
		})
		cancel()
		if err != nil {
			w.log.Error("load context", "chat", chat.JID, "err", err)
			continue
		}
		w.context.Load(chat.ID, msgs, w.cfg.CommandPrefix)
	}
	w.log.Info("context loaded", "chats", len(chats))
	return nil
}

// HandleStored decides whether a stored message needs an answer.
func (w *Worker) HandleStored(ctx context.Context, msg *bus.Message) {
	if msg.Outgoing && msg.Meta != nil && msg.Meta.ReplyFor != 0 {
		// Our answer reached the archive: link it to the usage record.
		if err := w.store.SetCompletionMessage(ctx, msg.Meta.ReplyFor, msg.ID); err != nil {
			w.log.Error("link usage", "err", err)
		}
		return
	}

	switch {
	case msg.Kind == bus.KindForAI:
	case (msg.Kind == bus.KindUser || msg.Kind == bus.KindPrivMsg) && !msg.Outgoing:
	default:
		return
	}

	if time.Since(msg.Time) > maxMessageAge {
		w.log.Info("skipping old message", "id", msg.ID, "time", msg.Time)
		return
	}

	w.enqueue(ctx, &Incoming{
		MessageID: msg.ID,
		ChatID:    msg.ChatID,
		Chat:      msg.Chat,
		Kind:      msg.Kind,
		Nick:      msg.Nick,
		Text:      msg.Text,
		Time:      msg.Time,
		Model:     w.cfg.Model,
	})
}

// enqueue processes messages of one chat in order, different chats in parallel.
func (w *Worker) enqueue(ctx context.Context, in *Incoming) {
	w.mu.Lock()
	q, ok := w.queues[in.ChatID]
	if !ok {
		q = make(chan *Incoming, 100)
		w.queues[in.ChatID] = q
		go func() {
			for in := range q {
				w.Process(ctx, in)
			}
		}()
	}
	w.mu.Unlock()

	select {
	case q <- in:
	default:
		w.log.Warn("chat queue is full, message dropped", "chat", in.ChatID)
	}
}

// Process runs the middleware chain and the model for one message.
func (w *Worker) Process(ctx context.Context, in *Incoming) {
	for _, mw := range w.chain {
		reply, drop := mw.Incoming(ctx, in)
		if drop {
			return
		}
		if reply != nil {
			w.send(ctx, reply)
			return
		}
	}

	w.log.Info("completion", "message", in.MessageID, "chat", in.Chat.JID, "model", in.Model, "context", len(in.Messages))

	out, err := w.complete(ctx, in)
	if err != nil {
		w.log.Error("completion failed", "message", in.MessageID, "err", err)
		w.send(ctx, in.Reply("AI error: "+err.Error()))
		return
	}

	for i := len(w.chain) - 1; i >= 0; i-- {
		if w.chain[i].Outgoing(ctx, out) {
			return
		}
	}
	w.send(ctx, out)
}

func (w *Worker) complete(ctx context.Context, in *Incoming) (*Outgoing, error) {
	maxTokens := w.cfg.TokensReservedForResponse

	cctx, cancel := context.WithTimeout(ctx, completionTimeout)
	res, err := w.completer.Complete(cctx, in.Model, in.Messages, maxTokens, in.Temperature)
	cancel()

	contextCleared := false
	if err != nil && ctx.Err() == nil {
		// Most failures are context overflows: retry with only the current message.
		w.log.Warn("completion failed, retrying with cleared context", "err", err)
		contextCleared = true
		cctx, cancel := context.WithTimeout(ctx, completionTimeout)
		res, err = w.completer.Complete(cctx, in.Model, w.context.Reset(in), maxTokens, in.Temperature)
		cancel()
	}
	if err != nil {
		return nil, err
	}

	text := res.Text
	if contextCleared {
		text = "[token limit exceeded, context was cleared] " + text
	}

	usage := res.Usage
	if err := w.store.AddUsage(ctx, UsageRecord{
		Time:             time.Now(),
		ChatID:           in.ChatID,
		ChatName:         in.Chat.Name,
		Nick:             in.Nick,
		Model:            res.Model,
		PromptMessageID:  in.MessageID,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}); err != nil {
		w.log.Error("store usage", "err", err)
	}

	out := in.Reply(text)
	out.Model = res.Model
	out.Usage = &usage
	out.Commands = in.Commands
	return out, nil
}

func (w *Worker) send(ctx context.Context, out *Outgoing) {
	cmd := &bus.SendCommand{
		ID:          uuid.NewString(),
		Chat:        out.Chat,
		PrivateNick: out.PrivateNick,
		Text:        out.Text,
	}
	if out.Model != "" {
		cmd.Meta = &bus.Meta{ReplyFor: out.ReplyFor, Model: out.Model, Usage: out.Usage}
	}
	if err := w.sender.Send(ctx, cmd); err != nil {
		w.log.Error("send answer", "chat", out.Chat.JID, "err", err)
	}
}

type jsSender struct {
	js jetstream.JetStream
}

func (s *jsSender) Send(ctx context.Context, cmd *bus.SendCommand) error {
	return bus.Publish(ctx, s.js, bus.SubjectSend, cmd.ID, cmd)
}

type openAICompleter struct {
	client *openai.Client
}

func (c *openAICompleter) Complete(ctx context.Context, model string, messages []ChatMessage, maxTokens int, temperature *float64) (*Completion, error) {
	params := openai.ChatCompletionNewParams{
		Model:               model,
		MaxCompletionTokens: openai.Int(int64(maxTokens)),
	}
	for _, m := range messages {
		if m.Role == "assistant" {
			params.Messages = append(params.Messages, openai.AssistantMessage(m.Content))
		} else {
			params.Messages = append(params.Messages, openai.UserMessage(m.Content))
		}
	}
	if temperature != nil {
		params.Temperature = openai.Float(*temperature)
	}

	res, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(res.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices")
	}

	return &Completion{
		Text:  res.Choices[0].Message.Content,
		Model: res.Model,
		Usage: bus.Usage{
			PromptTokens:     int(res.Usage.PromptTokens),
			CompletionTokens: int(res.Usage.CompletionTokens),
			TotalTokens:      int(res.Usage.TotalTokens),
		},
	}, nil
}
