// Package web serves the web interface: login, the single-page app and a
// WebSocket API on top of the history service and the bus.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/config"
)

//go:embed all:dist
var dist embed.FS

const (
	cookieName     = "session"
	requestTimeout = 20 * time.Second
	clientBuffer   = 256
)

type Server struct {
	cfg    config.WebUI
	nc     *nats.Conn
	js     jetstream.JetStream
	signer *Signer
	log    *slog.Logger

	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	out chan []byte
}

func Run(ctx context.Context, cfg config.WebUI, nc *nats.Conn) error {
	if len(cfg.PasswordsSHA512) == 0 {
		return errors.New("webui.passwords_sha512 is empty: nobody can log in")
	}
	signer, err := NewSigner(cfg.SigningKey, cfg.AuthExpiration)
	if err != nil {
		return err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := bus.EnsureStreams(ctx, js); err != nil {
		return err
	}

	s := &Server{
		cfg:     cfg,
		nc:      nc,
		js:      js,
		signer:  signer,
		log:     slog.With("service", "web-gateway"),
		clients: map[*client]struct{}{},
	}

	sub, err := nc.Subscribe(bus.SubjectStoredAll, s.broadcast)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Listen, strconv.Itoa(cfg.Port)),
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		s.log.Info("listening", "addr", "http://"+srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.Handle("/", spaHandler())
	return mux
}

func (s *Server) authenticated(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && s.signer.Check(c.Value)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": s.authenticated(r)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if !checkPassword(req.Password, s.cfg.PasswordsSHA512) {
		s.log.Warn("failed login", "remote", r.RemoteAddr)
		time.Sleep(time.Second) // slows down guessing
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong password"})
		return
	}

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    s.signer.Token(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	}
	if s.signer.expiration > 0 {
		cookie.MaxAge = int(s.signer.expiration.Seconds())
	}
	http.SetCookie(w, cookie)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// spaHandler serves the built frontend, falling back to index.html.
func spaHandler() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	files := http.FileServerFS(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(sub, path); err == nil {
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}

		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "frontend is not built: run `npm run build` in web/", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

// WebSocket API.
//
// Requests: {"id": 1, "type": "<method>", ...params}
// Responses: {"id": 1, "data": ...} or {"id": 1, "error": "..."}
// Pushes: {"type": "message", "data": <message>}

type wsRequest struct {
	ID     int64  `json:"id"`
	Type   string `json:"type"`
	ChatID int64  `json:"chat_id"`
	Date   string `json:"date"`
	TZ     string `json:"tz"`
	Text   string `json:"text"`
	ForAI  bool   `json:"for_ai"`
	Nick   string `json:"nick"`
	Color  string `json:"color"`
}

type wsResponse struct {
	ID    int64  `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The Vite dev server proxies from another origin.
		InsecureSkipVerify: s.cfg.Debug,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(1 << 20)
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &client{out: make(chan []byte, clientBuffer)}
	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
	}()

	go s.writeLoop(ctx, cancel, conn, c)

	for {
		var req wsRequest
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if err := json.Unmarshal(data, &req); err != nil {
			s.enqueue(c, wsResponse{Error: "invalid request"})
			continue
		}
		// Requests run concurrently: a slow query doesn't hold the others.
		go func() {
			rctx, rcancel := context.WithTimeout(ctx, requestTimeout)
			defer rcancel()
			resp := wsResponse{ID: req.ID}
			data, err := s.dispatch(rctx, &req)
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Data = data
			}
			s.enqueue(c, resp)
		}()
	}
}

func (s *Server) writeLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, c *client) {
	defer cancel()
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-c.out:
			wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, data)
			wcancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) enqueue(c *client, resp wsResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("marshal response", "err", err)
		return
	}
	select {
	case c.out <- data:
	default:
		s.log.Warn("client is too slow, response dropped")
	}
}

func (s *Server) broadcast(m *nats.Msg) {
	data, err := json.Marshal(wsResponse{Type: "message", Data: json.RawMessage(m.Data)})
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		select {
		case c.out <- data:
		default:
		}
	}
}

func (s *Server) dispatch(ctx context.Context, req *wsRequest) (any, error) {
	switch req.Type {
	case "chats":
		return bus.Request[[]bus.Chat](ctx, s.nc, bus.SubjectHistoryChats, struct{}{})
	case "dates":
		return bus.Request[[]string](ctx, s.nc, bus.SubjectHistoryDates, bus.DatesRequest{ChatID: req.ChatID, Timezone: req.TZ})
	case "messages":
		return bus.Request[[]bus.Message](ctx, s.nc, bus.SubjectHistoryMessages,
			bus.MessagesRequest{ChatID: req.ChatID, Date: req.Date, Timezone: req.TZ})
	case "nick_colors":
		return bus.Request[[]bus.NickColor](ctx, s.nc, bus.SubjectHistoryNickColors, struct{}{})
	case "set_nick_color":
		return bus.Request[string](ctx, s.nc, bus.SubjectHistorySetNickColor, bus.NickColor{Nick: req.Nick, Color: req.Color})
	case "send":
		return "OK", s.send(ctx, req)
	}
	return nil, fmt.Errorf("unknown request type %q", req.Type)
}

func (s *Server) send(ctx context.Context, req *wsRequest) error {
	text := strings.TrimSpace(req.Text)
	forAI := req.ForAI
	if rest, ok := strings.CutPrefix(text, "!!"); ok {
		text, forAI = strings.TrimSpace(rest), true
	}
	if text == "" {
		return errors.New("message is empty")
	}

	chat, err := bus.Request[bus.Chat](ctx, s.nc, bus.SubjectHistoryChat, bus.ChatRequest{ChatID: req.ChatID})
	if err != nil {
		return err
	}
	ref := bus.ChatRef{JID: chat.JID, Name: chat.Name, IsMUC: chat.IsMUC}
	id := uuid.NewString()

	if forAI {
		// Goes straight to the archive (and from there to the AI), not to XMPP.
		ev := &bus.Event{
			ID:       id,
			Time:     time.Now().UTC(),
			Kind:     bus.KindForAI,
			Chat:     ref,
			Nick:     "[FOR AI]",
			Text:     text,
			Outgoing: true,
		}
		return bus.Publish(ctx, s.js, ev.Subject(), id, ev)
	}

	return bus.Publish(ctx, s.js, bus.SubjectSend, id, &bus.SendCommand{ID: id, Chat: ref, Text: text})
}
