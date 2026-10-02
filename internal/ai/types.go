package ai

import (
	"context"
	"slices"
	"time"

	"github.com/h5vx/ugubot/internal/bus"
)

// ChatMessage is one entry of the conversation sent to the model.
type ChatMessage struct {
	Role    string `json:"role"` // user | assistant
	Content string `json:"content"`
}

// Incoming is a chat message travelling through the middleware chain.
type Incoming struct {
	MessageID int64
	ChatID    int64
	Chat      bus.ChatRef
	Kind      bus.Kind
	Nick      string
	Text      string
	Time      time.Time

	Commands    []string
	Model       string
	Temperature *float64
	// Conversation to send to the model, filled by ContextMiddleware.
	Messages []ChatMessage
}

func (in *Incoming) HasCommand(c string) bool {
	return slices.Contains(in.Commands, c)
}

// Reply creates a direct answer that bypasses the model.
func (in *Incoming) Reply(text string) *Outgoing {
	out := &Outgoing{
		ChatID:   in.ChatID,
		Chat:     in.Chat,
		ReplyFor: in.MessageID,
		Text:     text,
	}
	if in.Kind == bus.KindPrivMsg {
		out.PrivateNick = in.Nick
	}
	return out
}

// Outgoing is an answer to be sent to the chat.
type Outgoing struct {
	ChatID      int64
	Chat        bus.ChatRef
	PrivateNick string
	ReplyFor    int64
	Text        string
	// Model and Usage are set only for answers produced by the model.
	Model    string
	Usage    *bus.Usage
	Commands []string
}

func (out *Outgoing) HasCommand(c string) bool {
	return slices.Contains(out.Commands, c)
}

// Middleware processes messages before and after the model.
//
// Incoming returns a reply to answer directly without the model, or drop=true
// to stop processing silently. Outgoing is called in reverse order for model
// answers and may modify them; returning true drops the answer.
type Middleware interface {
	Incoming(ctx context.Context, in *Incoming) (reply *Outgoing, drop bool)
	Outgoing(ctx context.Context, out *Outgoing) (drop bool)
}

// base implements no-op Middleware methods for embedding.
type base struct{}

func (base) Incoming(context.Context, *Incoming) (*Outgoing, bool) { return nil, false }
func (base) Outgoing(context.Context, *Outgoing) bool              { return false }

// Pluralize picks a Russian plural form: Pluralize(n, "токен", "токенов", "токена").
func Pluralize(n int, singular, plural, plural2 string) string {
	switch {
	case n%10 == 1 && n%100 != 11:
		return singular
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return plural2
	}
	return plural
}
