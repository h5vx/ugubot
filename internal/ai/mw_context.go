package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/h5vx/ugubot/internal/bus"
)

type contextItem struct {
	msg       ChatMessage
	model     string
	tokens    int
	messageID int64 // stored message it came from, 0 for answers
}

type chatContext struct {
	items         []contextItem
	tokens        int
	prelude       string
	preludeTokens int
}

// Context keeps a separate conversation for each chat, limited by tokens, plus
// an optional per-chat prelude that is always sent first.
// Handles ~clear, ~prelude and ~context.
type Context struct {
	base
	Store        Store
	Tokens       *TokenCounter
	BotNick      string
	DefaultModel string
	MaxTokens    int // budget for prelude + context, without the reserved answer tokens

	mu    sync.Mutex
	chats map[int64]*chatContext
}

func (m *Context) chat(id int64) *chatContext {
	if m.chats == nil {
		m.chats = map[int64]*chatContext{}
	}
	c := m.chats[id]
	if c == nil {
		c = &chatContext{}
		m.chats[id] = c
	}
	return c
}

func (m *Context) Incoming(ctx context.Context, in *Incoming) (*Outgoing, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.chat(in.ChatID)

	if in.HasCommand("clear") {
		c.items, c.tokens = nil, 0
		if in.Text == "" {
			return in.Reply("Контекст очищен"), false
		}
	}
	if in.HasCommand("prelude") {
		return m.setPrelude(ctx, in, c), false
	}
	if in.HasCommand("context") {
		return in.Reply(m.describe(in, c)), false
	}

	msg := ChatMessage{Role: "user", Content: userContent(in.Chat, in.Kind, in.Nick, in.Text)}
	tokens := m.Tokens.Count(in.Model, msg)
	if c.preludeTokens+tokens > m.MaxTokens {
		return in.Reply(fmt.Sprintf("Сообщение слишком большое (%d %s)", tokens, Pluralize(tokens, "токен", "токенов", "токена"))), false
	}
	// A message that arrived while the context was being loaded may be there already.
	if !c.has(in.MessageID) {
		m.push(c, contextItem{msg: msg, model: in.Model, tokens: tokens, messageID: in.MessageID})
	}

	in.Messages = m.messages(c)
	return nil, false
}

func (m *Context) Outgoing(_ context.Context, out *Outgoing) bool {
	if out.Model == "" {
		return false // direct answers are not part of the conversation
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	tokens := 0
	if out.Usage != nil {
		tokens = out.Usage.CompletionTokens
	}
	m.push(m.chat(out.ChatID), contextItem{msg: ChatMessage{Role: "assistant", Content: out.Text}, model: out.Model, tokens: tokens})
	return false
}

// Reset clears the chat context leaving only the current message. It is used
// when the model rejects the request (usually because of the context size).
func (m *Context) Reset(in *Incoming) []ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.chat(in.ChatID)
	var last []contextItem
	if n := len(c.items); n > 0 {
		last = c.items[n-1:]
	}
	c.items, c.tokens = nil, 0
	for _, it := range last {
		m.push(c, it)
	}
	return m.messages(c)
}

// Load fills the context of a chat from stored messages (oldest first).
func (m *Context) Load(chatID int64, history []bus.Message, prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.chat(chatID)
	budget := m.MaxTokens - c.preludeTokens
	var items []contextItem

	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		var msg ChatMessage

		switch {
		case h.Outgoing && h.Kind == bus.KindUser:
			msg = ChatMessage{Role: "assistant", Content: h.Text}
		case h.Kind == bus.KindForAI:
			text, _ := parseCommands(strings.TrimSpace(h.Text), prefix, nil)
			msg = ChatMessage{Role: "user", Content: text}
		case h.Kind == bus.KindUser:
			text := strings.TrimSpace(h.Text)
			if h.Chat.IsMUC {
				var addressed bool
				if text, addressed = cutAddress(text, m.BotNick); !addressed {
					continue
				}
			}
			text, _ = parseCommands(text, prefix, nil)
			msg = ChatMessage{Role: "user", Content: userContent(h.Chat, h.Kind, h.Nick, text)}
		default:
			continue
		}

		tokens := m.Tokens.Count(m.DefaultModel, msg)
		if tokens > budget {
			break
		}
		budget -= tokens
		items = append(items, contextItem{msg: msg, model: m.DefaultModel, tokens: tokens, messageID: h.ID})
	}

	c.items, c.tokens = nil, 0
	for i := len(items) - 1; i >= 0; i-- {
		c.items = append(c.items, items[i])
		c.tokens += items[i].tokens
	}
	slog.Debug("context loaded", "chat", chatID, "messages", len(c.items), "tokens", c.tokens)
}

// LoadPrelude sets a prelude without persisting it.
func (m *Context) LoadPrelude(chatID int64, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.chat(chatID)
	c.prelude = text
	c.preludeTokens = m.preludeTokens(text)
}

func (m *Context) preludeTokens(text string) int {
	if text == "" {
		return 0
	}
	return m.Tokens.Count(m.DefaultModel, ChatMessage{Role: "user", Content: text})
}

func (m *Context) setPrelude(ctx context.Context, in *Incoming, c *chatContext) *Outgoing {
	tokens := m.preludeTokens(in.Text)
	plural := Pluralize(tokens, "токен", "токенов", "токена")

	if tokens > m.MaxTokens {
		return in.Reply(fmt.Sprintf("%s: Слишком большая прелюдия: %d %s из максимально возможных %d",
			in.Nick, tokens, plural, m.MaxTokens))
	}
	if err := m.Store.SetPrelude(ctx, in.ChatID, in.Text); err != nil {
		return in.Reply("Error: " + err.Error())
	}

	c.prelude, c.preludeTokens = in.Text, tokens
	for c.tokens+c.preludeTokens > m.MaxTokens && len(c.items) > 0 {
		m.dropOldest(c)
	}

	if in.Text == "" {
		return in.Reply(in.Nick + ": Прелюдия удалена")
	}
	return in.Reply(fmt.Sprintf("%s: Установлена новая прелюдия длиной в %d %s", in.Nick, tokens, plural))
}

func (m *Context) describe(in *Incoming, c *chatContext) string {
	if len(c.items) == 0 {
		return "Контекст пуст"
	}

	var lines []string
	line := func(n int, it contextItem) {
		s := fmt.Sprintf("%d: %s (%d tok)", n, shorten(strings.Join(strings.Fields(it.msg.Content), " "), 30), it.tokens)
		if !strings.HasPrefix(it.model, m.DefaultModel) {
			s += " [" + it.model + "]"
		}
		lines = append(lines, s)
	}

	if n := len(c.items); n > 6 {
		for i := range 4 {
			line(i+1, c.items[i])
		}
		lines = append(lines, fmt.Sprintf("< ... %d пропущено ... >", n-7))
		for i := n - 3; i < n; i++ {
			line(i+1, c.items[i])
		}
	} else {
		for i, it := range c.items {
			line(i+1, it)
		}
	}

	lines = append(lines, fmt.Sprintf("В контексте %d %s, для прелюдии используется %d %s",
		c.tokens, Pluralize(c.tokens, "токен", "токенов", "токена"),
		c.preludeTokens, Pluralize(c.preludeTokens, "токен", "токенов", "токена")))

	return in.Nick + ": Текущее содержимое контекста:\n" + strings.Join(lines, "\n")
}

func (m *Context) push(c *chatContext, it contextItem) {
	c.items = append(c.items, it)
	c.tokens += it.tokens
	for c.tokens+c.preludeTokens > m.MaxTokens && len(c.items) > 1 {
		m.dropOldest(c)
	}
}

func (c *chatContext) has(messageID int64) bool {
	if messageID == 0 {
		return false
	}
	for _, it := range c.items {
		if it.messageID == messageID {
			return true
		}
	}
	return false
}

func (m *Context) dropOldest(c *chatContext) {
	c.tokens -= c.items[0].tokens
	c.items = c.items[1:]
}

func (m *Context) messages(c *chatContext) []ChatMessage {
	msgs := make([]ChatMessage, 0, len(c.items)+1)
	if c.prelude != "" {
		msgs = append(msgs, ChatMessage{Role: "user", Content: c.prelude})
	}
	for _, it := range c.items {
		msgs = append(msgs, it.msg)
	}
	return msgs
}

// userContent prefixes room messages with the author's nick so the model can
// tell participants apart.
func userContent(chat bus.ChatRef, kind bus.Kind, nick, text string) string {
	if chat.IsMUC && kind == bus.KindUser {
		return nick + ": " + text
	}
	return text
}
