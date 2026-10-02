// Package bus describes everything services exchange over NATS: subjects,
// JetStream streams and message payloads.
package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nats.go/micro"
)

const (
	// Events produced by xmpp-gateway (and web-gateway for messages to AI).
	// Full subject: ugubot.xmpp.event.<kind>
	SubjectEventPrefix = "ugubot.xmpp.event."
	// Commands to send a message to XMPP.
	SubjectSend = "ugubot.xmpp.cmd.send"
	// Messages persisted by history. Full subject: ugubot.msg.stored.<chat_id>
	SubjectStoredPrefix = "ugubot.msg.stored."
	SubjectStoredAll    = SubjectStoredPrefix + ">"

	// Request/reply endpoints of the history service.
	SubjectHistoryChats        = "ugubot.history.chats"
	SubjectHistoryChat         = "ugubot.history.chat"
	SubjectHistoryMessages     = "ugubot.history.messages"
	SubjectHistoryDates        = "ugubot.history.dates"
	SubjectHistoryLastN        = "ugubot.history.last_n"
	SubjectHistoryNickColors   = "ugubot.history.nick_colors.get"
	SubjectHistorySetNickColor = "ugubot.history.nick_colors.set"

	StreamEvents   = "UGUBOT_EVENTS"
	StreamCommands = "UGUBOT_COMMANDS"
	StreamStored   = "UGUBOT_STORED"
)

// Kind of a chat message. String values are part of the web API.
type Kind string

const (
	KindForAI   Kind = "FOR_AI"
	KindUser    Kind = "USER"
	KindTopic   Kind = "TOPIC"
	KindJoin    Kind = "PART_JOIN"
	KindLeave   Kind = "PART_LEAVE"
	KindPrivMsg Kind = "MUC_PRIVMSG"
)

// Code is the numeric value stored in the database (kept from the Python version).
func (k Kind) Code() int16 {
	switch k {
	case KindForAI:
		return 0
	case KindUser:
		return 1
	case KindTopic:
		return 2
	case KindJoin:
		return 4
	case KindLeave:
		return 5
	case KindPrivMsg:
		return 6
	}
	return -1
}

func KindFromCode(c int16) Kind {
	switch c {
	case 0:
		return KindForAI
	case 1:
		return KindUser
	case 2:
		return KindTopic
	case 4:
		return KindJoin
	case 5:
		return KindLeave
	case 6:
		return KindPrivMsg
	}
	return Kind("UNKNOWN_" + strconv.Itoa(int(c)))
}

// ChatRef identifies a chat by its XMPP address. For MUC private messages the
// chat is the room itself.
type ChatRef struct {
	JID   string `json:"jid"`
	Name  string `json:"name"`
	IsMUC bool   `json:"is_muc"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Meta travels with an outgoing message from its author (ai-worker) through
// xmpp-gateway and history, so the author can recognise it in msg.stored.
type Meta struct {
	ReplyFor int64  `json:"reply_for,omitempty"`
	Model    string `json:"model,omitempty"`
	Usage    *Usage `json:"usage,omitempty"`
}

// Event is something that happened in a chat and must be stored.
type Event struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	Kind     Kind      `json:"kind"`
	Chat     ChatRef   `json:"chat"`
	Nick     string    `json:"nick"`
	Text     string    `json:"text,omitempty"`
	Outgoing bool      `json:"outgoing,omitempty"`
	Meta     *Meta     `json:"meta,omitempty"`
}

func (e *Event) Subject() string {
	return SubjectEventPrefix + string(e.Kind)
}

// SendCommand asks xmpp-gateway to send a message.
type SendCommand struct {
	ID   string  `json:"id"`
	Chat ChatRef `json:"chat"`
	// PrivateNick sends a private message to a room occupant instead of the room.
	PrivateNick string `json:"private_nick,omitempty"`
	Text        string `json:"text"`
	Meta        *Meta  `json:"meta,omitempty"`
}

// Message is a stored chat message.
type Message struct {
	ID       int64     `json:"id"`
	ChatID   int64     `json:"chat_id"`
	Chat     ChatRef   `json:"chat"`
	Time     time.Time `json:"time"`
	Kind     Kind      `json:"kind"`
	Nick     string    `json:"nick"`
	Text     string    `json:"text"`
	Outgoing bool      `json:"outgoing"`
	Meta     *Meta     `json:"meta,omitempty"`
}

func StoredSubject(chatID int64) string {
	return SubjectStoredPrefix + strconv.FormatInt(chatID, 10)
}

type Chat struct {
	ID    int64  `json:"id"`
	JID   string `json:"jid"`
	Name  string `json:"name"`
	IsMUC bool   `json:"is_muc"`
}

// Request payloads for history endpoints.

type ChatRequest struct {
	ChatID int64 `json:"chat_id"`
}

type MessagesRequest struct {
	ChatID   int64  `json:"chat_id"`
	Date     string `json:"date"` // YYYY-MM-DD in Timezone
	Timezone string `json:"tz"`
}

type DatesRequest struct {
	ChatID   int64  `json:"chat_id"`
	Timezone string `json:"tz"`
}

type LastNRequest struct {
	ChatID int64  `json:"chat_id"`
	N      int    `json:"n"`
	Kinds  []Kind `json:"kinds,omitempty"`
}

type NickColor struct {
	Nick  string `json:"nick"`
	Color string `json:"color"`
}

// ErrorResponse is returned by request/reply endpoints on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}

// EnsureStreams creates or updates the JetStream streams used by ugubot.
func EnsureStreams(ctx context.Context, js jetstream.JetStream) error {
	streams := []jetstream.StreamConfig{
		{
			Name:       StreamEvents,
			Subjects:   []string{SubjectEventPrefix + ">"},
			MaxAge:     7 * 24 * time.Hour,
			Duplicates: 10 * time.Minute,
		},
		{
			// Unsent commands older than an hour are not worth sending.
			Name:      StreamCommands,
			Subjects:  []string{SubjectSend},
			Retention: jetstream.WorkQueuePolicy,
			MaxAge:    time.Hour,
		},
		{
			Name:     StreamStored,
			Subjects: []string{SubjectStoredAll},
			MaxAge:   24 * time.Hour,
		},
	}

	for _, cfg := range streams {
		if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("stream %s: %w", cfg.Name, err)
		}
	}
	return nil
}

// Publish marshals v and publishes it to JetStream. msgID enables deduplication.
func Publish(ctx context.Context, js jetstream.JetStream, subject, msgID string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var opts []jetstream.PublishOpt
	if msgID != "" {
		opts = append(opts, jetstream.WithMsgID(msgID))
	}
	_, err = js.Publish(ctx, subject, data, opts...)
	return err
}

// Request performs a JSON request to a history endpoint.
func Request[Resp any](ctx context.Context, nc *nats.Conn, subject string, req any) (Resp, error) {
	var resp Resp

	data, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}

	msg, err := nc.RequestWithContext(ctx, subject, data)
	if err != nil {
		return resp, fmt.Errorf("%s: %w", subject, err)
	}

	if msg.Header.Get("Nats-Service-Error") != "" {
		return resp, fmt.Errorf("%s: %s", subject, msg.Header.Get("Nats-Service-Error"))
	}

	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return resp, fmt.Errorf("%s: decode response: %w", subject, err)
	}
	return resp, nil
}

// JSONHandler adapts a typed function to a NATS micro endpoint.
func JSONHandler[Req, Resp any](timeout time.Duration, fn func(context.Context, Req) (Resp, error)) micro.Handler {
	return micro.HandlerFunc(func(r micro.Request) {
		var req Req
		if len(r.Data()) > 0 {
			if err := json.Unmarshal(r.Data(), &req); err != nil {
				_ = r.Error("400", "invalid request: "+err.Error(), nil)
				return
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		resp, err := fn(ctx, req)
		if err != nil {
			_ = r.Error("500", err.Error(), nil)
			return
		}
		_ = r.RespondJSON(resp)
	})
}
