package ai

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/config"
)

type memStore struct {
	mu       sync.Mutex
	usage    []UsageRecord
	blocked  map[string]bool
	preludes map[int64]string
}

func newMemStore() *memStore {
	return &memStore{blocked: map[string]bool{}, preludes: map[int64]string{}}
}

func (s *memStore) AddUsage(_ context.Context, u UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage = append(s.usage, u)
	return nil
}
func (s *memStore) SetCompletionMessage(context.Context, int64, int64) error { return nil }
func (s *memStore) UsageSince(_ context.Context, since time.Time, chatID int64) ([]UsageRecord, error) {
	var out []UsageRecord
	for _, u := range s.usage {
		if !u.Time.Before(since) && (chatID == 0 || u.ChatID == chatID) {
			out = append(out, u)
		}
	}
	return out, nil
}
func (s *memStore) IsBlocked(_ context.Context, who string) (bool, error) { return s.blocked[who], nil }
func (s *memStore) Block(_ context.Context, who string) error             { s.blocked[who] = true; return nil }
func (s *memStore) Unblock(_ context.Context, who string) (bool, error) {
	ok := s.blocked[who]
	delete(s.blocked, who)
	return ok, nil
}
func (s *memStore) Blocklist(context.Context) ([]string, error) {
	var out []string
	for k := range s.blocked {
		out = append(out, k)
	}
	return out, nil
}
func (s *memStore) Preludes(context.Context) (map[int64]string, error) { return s.preludes, nil }
func (s *memStore) SetPrelude(_ context.Context, id int64, text string) error {
	s.preludes[id] = text
	return nil
}

type fakeCompleter struct {
	calls    [][]ChatMessage
	temps    []*float64
	failures int
}

func (f *fakeCompleter) Complete(_ context.Context, model string, msgs []ChatMessage, _ int, t *float64) (*Completion, error) {
	f.calls = append(f.calls, append([]ChatMessage(nil), msgs...))
	f.temps = append(f.temps, t)
	if f.failures > 0 {
		f.failures--
		return nil, errors.New("context_length_exceeded")
	}
	return &Completion{
		Text:  "answer",
		Model: model,
		Usage: bus.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500},
	}, nil
}

type fakeSender struct {
	sent []*bus.SendCommand
}

func (f *fakeSender) Send(_ context.Context, cmd *bus.SendCommand) error {
	f.sent = append(f.sent, cmd)
	return nil
}

var (
	room   = bus.ChatRef{JID: "room@conference.example.com", Name: "room@conference.example.com", IsMUC: true}
	direct = bus.ChatRef{JID: "alice@example.com", Name: "alice"}
)

func testConfig() config.OpenAI {
	return config.OpenAI{
		Enabled:                   true,
		Model:                     "gpt-4o-mini",
		ModelSecondaryCommand:     "4",
		ModelSecondary:            "gpt-4o",
		UserNick:                  "bot",
		CommandPrefix:             "~",
		MaxTokens:                 4096,
		TokensReservedForResponse: 512,
		Prompt:                    map[string]config.Prompt{"dan": {Command: "dan", Text: "Be DAN."}},
		Prices:                    map[string]config.Price{"gpt-4o": {Input: 0.01, Output: 0.02}, "gpt-4o-mini": {Input: 0.001, Output: 0.002}},
	}
}

type harness struct {
	w      *Worker
	store  *memStore
	model  *fakeCompleter
	sender *fakeSender
	nextID int64
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{store: newMemStore(), model: &fakeCompleter{}, sender: &fakeSender{}}
	h.w = NewWorker(testConfig(), []string{"admin@example.com"}, h.store, h.model, h.sender)
	return h
}

func (h *harness) say(chat bus.ChatRef, kind bus.Kind, nick, text string) *bus.SendCommand {
	h.nextID++
	before := len(h.sender.sent)
	h.w.Process(context.Background(), &Incoming{
		MessageID: h.nextID, ChatID: chatID(chat), Chat: chat, Kind: kind,
		Nick: nick, Text: text, Time: time.Now(), Model: h.w.cfg.Model,
	})
	if len(h.sender.sent) == before {
		return nil
	}
	return h.sender.sent[len(h.sender.sent)-1]
}

func chatID(c bus.ChatRef) int64 {
	if c.IsMUC {
		return 1
	}
	return 2
}

func TestRoomMessageMustBeAddressed(t *testing.T) {
	h := newHarness(t)

	if out := h.say(room, bus.KindUser, "alice", "hello everyone"); out != nil {
		t.Fatalf("unaddressed message answered: %+v", out)
	}

	out := h.say(room, bus.KindUser, "alice", "bot: what is XMPP?")
	if out == nil || out.Text != "answer" || out.Chat != room || out.PrivateNick != "" {
		t.Fatalf("unexpected answer: %+v", out)
	}
	if out.Meta == nil || out.Meta.ReplyFor != 2 || out.Meta.Model != "gpt-4o-mini" {
		t.Fatalf("answer meta: %+v", out.Meta)
	}
	last := h.model.calls[0]
	if got := last[len(last)-1]; got.Role != "user" || got.Content != "alice: what is XMPP?" {
		t.Fatalf("prompt: %+v", got)
	}
	if len(h.store.usage) != 1 || h.store.usage[0].PromptMessageID != 2 {
		t.Fatalf("usage not recorded: %+v", h.store.usage)
	}
}

func TestContextAccumulates(t *testing.T) {
	h := newHarness(t)
	h.say(direct, bus.KindUser, "alice", "first")
	h.say(direct, bus.KindUser, "alice", "second")

	got := h.model.calls[1]
	want := []ChatMessage{{"user", "first"}, {"assistant", "answer"}, {"user", "second"}}
	if len(got) != len(want) {
		t.Fatalf("context: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("context[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if out := h.say(direct, bus.KindUser, "alice", "~clear"); out == nil || out.Text != "Контекст очищен" {
		t.Fatalf("clear: %+v", out)
	}
	h.say(direct, bus.KindUser, "alice", "third")
	if got := h.model.calls[2]; len(got) != 1 {
		t.Fatalf("context after clear: %+v", got)
	}
}

func TestPrivateRoomMessageIsAnsweredPrivately(t *testing.T) {
	h := newHarness(t)
	out := h.say(room, bus.KindPrivMsg, "alice", "hi there")
	if out == nil || out.PrivateNick != "alice" || out.Chat != room {
		t.Fatalf("answer: %+v", out)
	}
}

func TestDirectCommands(t *testing.T) {
	h := newHarness(t)

	if out := h.say(direct, bus.KindUser, "alice", "~help"); out == nil || !strings.Contains(out.Text, "~prelude") || out.Meta != nil {
		t.Fatalf("help: %+v", out)
	}
	if out := h.say(direct, bus.KindUser, "alice", "~t abc hello"); out == nil || !strings.Contains(out.Text, "not a valid number") {
		t.Fatalf("bad temperature: %+v", out)
	}
	if out := h.say(direct, bus.KindUser, "alice", "~t 3 hello"); out == nil || !strings.Contains(out.Text, "range 0 - 2") {
		t.Fatalf("temperature range: %+v", out)
	}
	if len(h.model.calls) != 0 {
		t.Fatalf("model called for commands: %d", len(h.model.calls))
	}

	h.say(direct, bus.KindUser, "alice", "~4 ~t 0.7 hello")
	if temp := h.model.temps[0]; temp == nil || *temp != 0.7 {
		t.Fatalf("temperature not passed: %v", temp)
	}
	if u := h.store.usage[0]; u.Model != "gpt-4o" {
		t.Fatalf("secondary model not used: %s", u.Model)
	}
}

func TestPreludeAndUserPrompt(t *testing.T) {
	h := newHarness(t)

	if out := h.say(direct, bus.KindUser, "alice", "~prelude You are a cat."); out == nil || !strings.Contains(out.Text, "Установлена новая прелюдия") {
		t.Fatalf("prelude: %+v", out)
	}
	if h.store.preludes[2] != "You are a cat." {
		t.Fatalf("prelude not persisted: %q", h.store.preludes[2])
	}

	h.say(direct, bus.KindUser, "alice", "~dan hi")
	got := h.model.calls[0]
	if got[0].Content != "You are a cat." || got[len(got)-1].Content != "Be DAN. hi" {
		t.Fatalf("messages: %+v", got)
	}
}

func TestRetryClearsContext(t *testing.T) {
	h := newHarness(t)
	h.say(direct, bus.KindUser, "alice", "one")
	h.model.failures = 1

	out := h.say(direct, bus.KindUser, "alice", "two")
	if out == nil || !strings.HasPrefix(out.Text, "[token limit exceeded, context was cleared] ") {
		t.Fatalf("answer: %+v", out)
	}
	retry := h.model.calls[len(h.model.calls)-1]
	if len(retry) != 1 || retry[0].Content != "two" {
		t.Fatalf("retry messages: %+v", retry)
	}
}

func TestInlineUsageAndReport(t *testing.T) {
	h := newHarness(t)

	out := h.say(direct, bus.KindUser, "alice", "~$ hello")
	if out == nil || !strings.HasPrefix(out.Text, "[$0.0020 (IN $0.0010 / OUT $0.0010, gpt-4o-mini)] answer") {
		t.Fatalf("inline usage: %+v", out)
	}

	h.say(room, bus.KindUser, "bob", "bot, ~4 hi")

	report := h.say(room, bus.KindUser, "bob", "bot: ~usage global")
	if report == nil || !strings.Contains(report.Text, "по всем чатам") ||
		!strings.Contains(report.Text, "alice") || !strings.Contains(report.Text, "room@conference.example.com") {
		t.Fatalf("global report:\n%s", report.Text)
	}

	local := h.say(room, bus.KindUser, "bob", "bot: ~usage 7")
	if !strings.Contains(local.Text, "за последние 7 дней") || !strings.Contains(local.Text, "bob") || strings.Contains(local.Text, "alice") {
		t.Fatalf("chat report:\n%s", local.Text)
	}
	if !strings.Contains(local.Text, "$0.02") { // gpt-4o: 1000*0.01/1000 + 500*0.02/1000
		t.Fatalf("chat report cost:\n%s", local.Text)
	}
}

func TestBlocklist(t *testing.T) {
	h := newHarness(t)
	admin := bus.ChatRef{JID: "admin@example.com", Name: "admin"}

	if out := h.say(direct, bus.KindUser, "alice", "~block bob"); !strings.Contains(out.Text, "don't have access") {
		t.Fatalf("non-admin block: %+v", out)
	}
	if out := h.say(room, bus.KindUser, "x", "bot: ~block bob"); !strings.Contains(out.Text, "only in private") {
		t.Fatalf("block in room: %+v", out)
	}
	if out := h.say(admin, bus.KindUser, "admin", "~block bob"); out.Text != "bob is added to blocklist" {
		t.Fatalf("admin block: %+v", out)
	}
	if out := h.say(room, bus.KindUser, "bob", "bot: hi"); out != nil {
		t.Fatalf("blocked user answered: %+v", out)
	}
	if out := h.say(admin, bus.KindUser, "admin", "~unblock bob"); out.Text != "bob is removed from blocklist" {
		t.Fatalf("unblock: %+v", out)
	}
}

func TestDisabled(t *testing.T) {
	h := newHarness(t)
	h.w.chain[0] = &DropIfDisabled{Enabled: false}
	if out := h.say(direct, bus.KindUser, "alice", "~help"); out != nil {
		t.Fatalf("answered while disabled: %+v", out)
	}
}

func TestContextLoad(t *testing.T) {
	h := newHarness(t)
	history := []bus.Message{
		{Chat: room, Kind: bus.KindUser, Nick: "alice", Text: "unrelated chatter"},
		{Chat: room, Kind: bus.KindUser, Nick: "alice", Text: "bot: ~4 question"},
		{Chat: room, Kind: bus.KindUser, Nick: "bot", Text: "reply", Outgoing: true},
		{Chat: room, Kind: bus.KindForAI, Nick: "[FOR AI]", Text: "from web", Outgoing: true},
	}
	h.w.context.Load(1, history, "~")

	h.say(room, bus.KindUser, "alice", "bot: next")
	got := h.model.calls[0]
	want := []string{"alice: question", "reply", "from web", "alice: next"}
	if len(got) != len(want) {
		t.Fatalf("messages: %+v", got)
	}
	for i := range want {
		if got[i].Content != want[i] {
			t.Fatalf("message %d = %q, want %q", i, got[i].Content, want[i])
		}
	}
}

func TestContextIsTrimmedByTokens(t *testing.T) {
	h := newHarness(t)
	h.w.context.MaxTokens = 60
	for range 10 {
		h.say(direct, bus.KindUser, "alice", "some words here")
	}
	last := h.model.calls[len(h.model.calls)-1]
	if n := h.w.context.Tokens.Count("gpt-4o-mini", last...); n > 60 {
		t.Fatalf("context has %d tokens, limit 60", n)
	}
	if last[len(last)-1].Content != "some words here" {
		t.Fatalf("current message missing: %+v", last)
	}
}

func TestPluralize(t *testing.T) {
	cases := map[int]string{0: "токенов", 1: "токен", 2: "токена", 5: "токенов", 11: "токенов", 12: "токенов", 21: "токен", 22: "токена", 111: "токенов", 104: "токена"}
	for n, want := range cases {
		if got := Pluralize(n, "токен", "токенов", "токена"); got != want {
			t.Errorf("Pluralize(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestParseCommands(t *testing.T) {
	text, cmds := parseCommands("~clear ~dan  hello world", "~", nil)
	if text != " hello world" || len(cmds) != 2 || cmds[0] != "clear" || cmds[1] != "dan" {
		t.Fatalf("got %q %v", text, cmds)
	}
	text, cmds = parseCommands("~help", "~", nil)
	if text != "" || len(cmds) != 1 {
		t.Fatalf("got %q %v", text, cmds)
	}
}

func TestMessageLoadedIntoContextIsNotDuplicated(t *testing.T) {
	h := newHarness(t)
	h.w.context.Load(2, []bus.Message{{ID: 1, Chat: direct, Kind: bus.KindUser, Nick: "alice", Text: "question"}}, "~")

	h.say(direct, bus.KindUser, "alice", "question") // same message id 1, delivered after loading
	if got := h.model.calls[0]; len(got) != 1 {
		t.Fatalf("message duplicated in context: %+v", got)
	}
}
