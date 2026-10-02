// Command e2e drives a running ugubot through a real XMPP server and the web
// API, playing users and a fake OpenAI endpoint. See run.sh.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/coder/websocket"
	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"
)

var (
	xmppHost = env("E2E_XMPP_HOST", "ugu-prosody:5222")
	webURL   = env("E2E_WEB_URL", "http://ugu-bot:8000")
	webPass  = env("E2E_WEB_PASSWORD", "webpass")
	room     = "test@conference.localhost"
	botNick  = "ugubot"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ai := startFakeOpenAI(":9999")

	web := loginWeb(ctx)
	log.Println("web login ok")

	alice := connect(ctx, "alice@localhost/e2e", "alicepass")
	alice.join(ctx, room, "alice")
	alice.waitPresence(ctx, room+"/"+botNick)
	log.Println("alice sees the bot in the room")

	alice.groupchat(ctx, room, "hello room")
	alice.groupchat(ctx, room, botNick+": what is 2+2?")
	m := alice.waitMessage(ctx, func(m msg) bool { return m.From == room+"/"+botNick })
	expect(m.Body == "fake answer to: alice: what is 2+2?", "room answer: %q", m.Body)
	log.Println("bot answered in the room")

	alice.chat(ctx, "bot@localhost", "hi in private")
	m = alice.waitMessage(ctx, func(m msg) bool { return strings.HasPrefix(m.From, "bot@localhost") })
	expect(m.Body == "fake answer to: hi in private", "private answer: %q", m.Body)
	log.Println("bot answered privately")

	alice.chat(ctx, room+"/"+botNick, "~help")
	m = alice.waitMessage(ctx, func(m msg) bool { return m.From == room+"/"+botNick && m.Type == "chat" })
	expect(strings.Contains(m.Body, "Команды начинаются"), "room private answer: %q", m.Body)
	log.Println("bot answered a private room message privately")

	alice.topic(ctx, room, "e2e topic")

	bob := connect(ctx, "bob@localhost/e2e", "bobpass")
	bob.join(ctx, room, "bob")
	alice.waitPresence(ctx, room+"/bob")
	time.Sleep(500 * time.Millisecond)
	bob.leave(ctx, room, "bob")
	time.Sleep(2 * time.Second) // let history store the events

	// Web API.
	ws := web.ws(ctx)
	chats := ws.call(ctx, map[string]any{"type": "chats"})
	var chatList []struct {
		ID    int64  `json:"id"`
		JID   string `json:"jid"`
		IsMUC bool   `json:"is_muc"`
	}
	mustUnmarshal(chats, &chatList)
	var roomID int64
	for _, c := range chatList {
		if c.JID == room {
			roomID = c.ID
		}
	}
	expect(roomID != 0 && len(chatList) == 2, "chats: %s", chats)

	tz := "Asia/Kathmandu"
	loc, _ := time.LoadLocation(tz)
	today := time.Now().In(loc).Format(time.DateOnly)

	var dates []string
	mustUnmarshal(ws.call(ctx, map[string]any{"type": "dates", "chat_id": roomID, "tz": tz}), &dates)
	expect(slices.Equal(dates, []string{today}), "dates: %v, today %s", dates, today)

	type webMsg struct {
		ID       int64  `json:"id"`
		Kind     string `json:"kind"`
		Nick     string `json:"nick"`
		Text     string `json:"text"`
		Outgoing bool   `json:"outgoing"`
	}
	var msgs []webMsg
	mustUnmarshal(ws.call(ctx, map[string]any{"type": "messages", "chat_id": roomID, "date": today, "tz": tz}), &msgs)
	var got []string
	for _, m := range msgs {
		got = append(got, fmt.Sprintf("%s %s %q out=%v", m.Kind, m.Nick, m.Text, m.Outgoing))
	}
	want := []string{
		`PART_JOIN alice "" out=false`,
		`USER alice "hello room" out=false`,
		`USER alice "ugubot: what is 2+2?" out=false`,
		`USER ugubot "fake answer to: alice: what is 2+2?" out=true`,
		`MUC_PRIVMSG alice "~help" out=false`,
		`TOPIC alice "e2e topic" out=false`,
		`PART_JOIN bob "" out=false`,
		`PART_LEAVE bob "NORMAL" out=false`,
	}
	// The ~help answer is a private room message too.
	got = slices.DeleteFunc(got, func(s string) bool { return strings.HasPrefix(s, "MUC_PRIVMSG alice \"Команды") })
	expect(slices.Equal(got, want), "room messages:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	log.Println("history is correct")

	// Sending from the web goes to XMPP and comes back as a push.
	ws.call(ctx, map[string]any{"type": "send", "chat_id": roomID, "text": "from web"})
	m = alice.waitMessage(ctx, func(m msg) bool { return m.Body == "from web" })
	expect(m.From == room+"/"+botNick, "web message from %s", m.From)
	push := ws.waitPush(ctx, func(p json.RawMessage) bool { return strings.Contains(string(p), `"from web"`) })
	expect(strings.Contains(string(push), `"outgoing":true`), "push: %s", push)
	log.Println("web message delivered and pushed")

	// "!!" goes only to the AI; the answer goes to the room.
	ws.call(ctx, map[string]any{"type": "send", "chat_id": roomID, "text": "!!question from web"})
	m = alice.waitMessage(ctx, func(m msg) bool { return strings.HasPrefix(m.Body, "fake answer to: question from web") })
	log.Println("AI answered a web-only question:", m.Body)

	expect(ai.calls() == 3, "openai calls: %d", ai.calls())
	log.Println("E2E OK")
}

func expect(ok bool, format string, args ...any) {
	if !ok {
		log.Fatalf("FAIL: "+format, args...)
	}
}

func mustUnmarshal(data json.RawMessage, v any) {
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("decode %s: %v", data, err)
	}
}

// Fake OpenAI.

type fakeAI struct {
	mu sync.Mutex
	n  int
}

func (f *fakeAI) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n
}

func startFakeOpenAI(addr string) *fakeAI {
	f := &fakeAI{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.n++
		f.mu.Unlock()
		last := req.Messages[len(req.Messages)-1].Content
		log.Printf("openai: %d messages, last %q", len(req.Messages), last)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "x", "object": "chat.completion", "created": time.Now().Unix(), "model": req.Model,
			"choices": []any{map[string]any{
				"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": "fake answer to: " + last},
			}},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		})
	})
	go func() { log.Fatal(http.ListenAndServe(addr, mux)) }()
	return f
}

// Web client.

type webClient struct {
	http *http.Client
}

func loginWeb(ctx context.Context) *webClient {
	jar, _ := cookiejar.New(nil)
	c := &webClient{http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
	body, _ := json.Marshal(map[string]string{"password": webPass})

	for {
		res, err := c.http.Post(webURL+"/api/login", "application/json", bytes.NewReader(body))
		if err == nil {
			res.Body.Close()
			expect(res.StatusCode == 200, "login status %d", res.StatusCode)
			return c
		}
		if ctx.Err() != nil {
			log.Fatalf("web is not up: %v", err)
		}
		time.Sleep(time.Second)
	}
}

type wsClient struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	nextID int
	resp   map[int]chan json.RawMessage
	pushes chan json.RawMessage
}

func (c *webClient) ws(ctx context.Context) *wsClient {
	u, _ := url.Parse(webURL)
	conn, _, err := websocket.Dial(ctx, "ws://"+u.Host+"/ws", &websocket.DialOptions{HTTPClient: c.http})
	if err != nil {
		log.Fatalf("ws dial: %v", err)
	}
	w := &wsClient{conn: conn, resp: map[int]chan json.RawMessage{}, pushes: make(chan json.RawMessage, 100)}
	go func() {
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var m struct {
				ID    int             `json:"id"`
				Type  string          `json:"type"`
				Data  json.RawMessage `json:"data"`
				Error string          `json:"error"`
			}
			_ = json.Unmarshal(data, &m)
			if m.Error != "" {
				log.Fatalf("ws error: %s", m.Error)
			}
			if m.Type == "message" {
				w.pushes <- m.Data
				continue
			}
			w.mu.Lock()
			ch := w.resp[m.ID]
			w.mu.Unlock()
			if ch != nil {
				ch <- m.Data
			}
		}
	}()
	return w
}

func (w *wsClient) call(ctx context.Context, req map[string]any) json.RawMessage {
	w.mu.Lock()
	w.nextID++
	id := w.nextID
	ch := make(chan json.RawMessage, 1)
	w.resp[id] = ch
	w.mu.Unlock()

	req["id"] = id
	data, _ := json.Marshal(req)
	if err := w.conn.Write(ctx, websocket.MessageText, data); err != nil {
		log.Fatalf("ws write: %v", err)
	}
	select {
	case r := <-ch:
		return r
	case <-ctx.Done():
		log.Fatalf("ws %v: timeout", req["type"])
	}
	return nil
}

func (w *wsClient) waitPush(ctx context.Context, match func(json.RawMessage) bool) json.RawMessage {
	for {
		select {
		case p := <-w.pushes:
			if match(p) {
				return p
			}
		case <-ctx.Done():
			log.Fatal("timeout waiting for push")
		}
	}
}

// XMPP client.

type msg struct {
	From, Type, Body string
}

type user struct {
	s         *xmpp.Session
	messages  chan msg
	mu        sync.Mutex
	presences map[string]bool
}

func connect(ctx context.Context, addr, pass string) *user {
	j := jid.MustParse(addr)
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", xmppHost)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	s, err := xmpp.NewClientSession(ctx, j, conn,
		xmpp.StartTLS(&tls.Config{ServerName: j.Domainpart(), InsecureSkipVerify: true}),
		xmpp.SASL("", pass, sasl.ScramSha1, sasl.Plain),
		xmpp.BindResource(),
	)
	if err != nil {
		log.Fatalf("%s session: %v", addr, err)
	}
	u := &user{s: s, messages: make(chan msg, 100), presences: map[string]bool{}}

	go s.Serve(xmpp.HandlerFunc(func(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
		// The start element is already consumed: put it back so the decoder
		// sees a balanced element.
		d := xml.NewTokenDecoder(xmlstream.MultiReader(xmlstream.Token(*start), t))
		switch start.Name.Local {
		case "message":
			var m struct {
				stanza.Message
				Body string `xml:"body"`
			}
			if err := d.Decode(&m); err != nil && !errors.Is(err, io.EOF) {
				return nil
			}
			if m.Body != "" {
				u.messages <- msg{From: m.From.String(), Type: string(m.Type), Body: m.Body}
			}
		case "presence":
			var p stanza.Presence
			if err := d.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
				return nil
			}
			u.mu.Lock()
			u.presences[p.From.String()] = p.Type != stanza.UnavailablePresence
			u.mu.Unlock()
		}
		return nil
	}))

	if err := s.Encode(ctx, struct {
		XMLName xml.Name `xml:"presence"`
	}{}); err != nil {
		log.Fatal(err)
	}
	return u
}

func (u *user) join(ctx context.Context, room, nick string) {
	err := u.s.Encode(ctx, struct {
		XMLName xml.Name `xml:"presence"`
		To      string   `xml:"to,attr"`
		X       struct {
			XMLName xml.Name `xml:"http://jabber.org/protocol/muc x"`
		}
	}{To: room + "/" + nick})
	if err != nil {
		log.Fatal(err)
	}
}

func (u *user) leave(ctx context.Context, room, nick string) {
	err := u.s.Encode(ctx, struct {
		XMLName xml.Name `xml:"presence"`
		To      string   `xml:"to,attr"`
		Type    string   `xml:"type,attr"`
	}{To: room + "/" + nick, Type: "unavailable"})
	if err != nil {
		log.Fatal(err)
	}
}

func (u *user) waitPresence(ctx context.Context, from string) {
	for {
		u.mu.Lock()
		ok := u.presences[from]
		u.mu.Unlock()
		if ok {
			return
		}
		select {
		case <-ctx.Done():
			log.Fatalf("timeout waiting for presence of %s", from)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (u *user) send(ctx context.Context, to, typ, body, subject string) {
	err := u.s.Encode(ctx, struct {
		XMLName xml.Name `xml:"message"`
		To      string   `xml:"to,attr"`
		Type    string   `xml:"type,attr"`
		Body    string   `xml:"body,omitempty"`
		Subject string   `xml:"subject,omitempty"`
	}{To: to, Type: typ, Body: body, Subject: subject})
	if err != nil {
		log.Fatal(err)
	}
}

func (u *user) groupchat(ctx context.Context, room, body string) {
	u.send(ctx, room, "groupchat", body, "")
}
func (u *user) chat(ctx context.Context, to, body string) { u.send(ctx, to, "chat", body, "") }
func (u *user) topic(ctx context.Context, room, subject string) {
	u.send(ctx, room, "groupchat", "", subject)
}

func (u *user) waitMessage(ctx context.Context, match func(msg) bool) msg {
	for {
		select {
		case m := <-u.messages:
			if match(m) {
				return m
			}
		case <-ctx.Done():
			log.Fatal("timeout waiting for message")
		}
	}
}
