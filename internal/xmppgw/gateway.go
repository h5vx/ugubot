// Package xmppgw holds the only XMPP session of the bot. It turns stanzas into
// bus events and executes send commands. It has no database access.
package xmppgw

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/dial"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/ping"
	"mellium.im/xmpp/stanza"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/config"
)

const (
	consumerName = "xmpp-gateway"
	pingInterval = time.Minute
	pingTimeout  = 30 * time.Second
	// Servers send the room subject last when joining (XEP-0045 §7.2.15).
	// If it never comes, consider the room joined after this delay.
	joinTimeout  = 15 * time.Second
	rejoinDelay  = 30 * time.Second
	maxReconnect = time.Minute
)

var errNotConnected = errors.New("xmpp is not connected")

type roomState int

const (
	roomLeft roomState = iota
	roomJoining
	roomJoined
)

type room struct {
	cfg       config.Room
	jid       jid.JID
	myNick    string
	state     roomState
	occupants map[string]struct{}
}

type Gateway struct {
	cfg config.XMPP
	js  jetstream.JetStream
	log *slog.Logger
	me  jid.JID

	events chan *bus.Event

	mu      sync.Mutex
	session *xmpp.Session
	rooms   map[string]*room // keyed by bare room JID
}

func Run(ctx context.Context, cfg config.XMPP, nc *nats.Conn) error {
	me, err := jid.Parse(cfg.JID)
	if err != nil {
		return fmt.Errorf("xmpp.jid: %w", err)
	}
	if cfg.Resource != "" && me.Resourcepart() == "" {
		if me, err = me.WithResource(cfg.Resource); err != nil {
			return fmt.Errorf("xmpp.resource: %w", err)
		}
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return err
	}
	if err := bus.EnsureStreams(ctx, js); err != nil {
		return err
	}

	g := &Gateway{
		cfg:    cfg,
		js:     js,
		log:    slog.With("service", "xmpp-gateway"),
		me:     me,
		events: make(chan *bus.Event, 4096),
		rooms:  map[string]*room{},
	}

	for name, rc := range cfg.Rooms {
		if !rc.Join {
			continue
		}
		rj, err := jid.Parse(rc.JID)
		if err != nil {
			return fmt.Errorf("xmpp.rooms.%s.jid: %w", name, err)
		}
		if rc.Nick == "" {
			rc.Nick = me.Localpart()
		}
		g.rooms[rj.Bare().String()] = &room{cfg: rc, jid: rj.Bare(), myNick: rc.Nick, occupants: map[string]struct{}{}}
	}

	go g.publishEvents(ctx)

	consumer, err := js.CreateOrUpdateConsumer(ctx, bus.StreamCommands, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    20,
		MaxAckPending: 1, // keep message order
	})
	if err != nil {
		return fmt.Errorf("consumer: %w", err)
	}
	cc, err := consumer.Consume(g.handleCommand)
	if err != nil {
		return err
	}
	defer cc.Stop()

	backoff := 2 * time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := g.runSession(ctx)
		if ctx.Err() != nil {
			break
		}
		if time.Since(started) > 5*time.Minute {
			backoff = 2 * time.Second
		}
		g.log.Error("xmpp session ended, reconnecting", "err", err, "in", backoff)
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxReconnect)
	}
	return nil
}

func (g *Gateway) runSession(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var conn net.Conn
	var err error
	if g.cfg.Host != "" {
		conn, err = (&net.Dialer{}).DialContext(dialCtx, "tcp", g.cfg.Host)
	} else {
		conn, err = (&dial.Dialer{NoTLS: true}).Dial(dialCtx, "tcp", g.me)
	}
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	tlsConfig := &tls.Config{
		ServerName:         g.me.Domainpart(),
		InsecureSkipVerify: !g.cfg.SSLVerify, //nolint:gosec // explicit opt-out in settings
		MinVersion:         tls.VersionTLS12,
	}

	s, err := xmpp.NewClientSession(dialCtx, g.me, conn,
		xmpp.StartTLS(tlsConfig),
		xmpp.SASL("", g.cfg.Password, sasl.ScramSha256Plus, sasl.ScramSha1Plus, sasl.ScramSha256, sasl.ScramSha1, sasl.Plain),
		xmpp.BindResource(),
	)
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}
	defer s.Close()

	g.log.Info("connected", "jid", s.LocalAddr().String())

	if err := s.Encode(ctx, outPresence{}); err != nil {
		return fmt.Errorf("initial presence: %w", err)
	}

	g.mu.Lock()
	g.session = s
	for _, r := range g.rooms {
		g.joinLocked(ctx, s, r)
	}
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		g.session = nil
		for _, r := range g.rooms {
			r.state = roomLeft
		}
		g.mu.Unlock()
	}()

	sessCtx, stop := context.WithCancel(ctx)
	defer stop()
	go g.keepAlive(sessCtx, s, conn)
	go func() {
		<-sessCtx.Done()
		_ = conn.Close() // unblocks Serve on shutdown
	}()

	return s.Serve(xmpp.HandlerFunc(g.handle))
}

func (g *Gateway) keepAlive(ctx context.Context, s *xmpp.Session, conn net.Conn) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, pingTimeout)
		err := ping.Send(pctx, s, g.me.Domain())
		cancel()
		if err != nil && ctx.Err() == nil {
			g.log.Warn("ping failed, dropping connection", "err", err)
			_ = conn.Close()
			return
		}
	}
}

// joinLocked sends a join presence. Must be called with g.mu held and never
// from inside the stanza handler (session send methods would deadlock there).
func (g *Gateway) joinLocked(ctx context.Context, s *xmpp.Session, r *room) {
	r.state = roomJoining
	r.myNick = r.cfg.Nick
	clear(r.occupants)

	to, err := r.jid.WithResource(r.cfg.Nick)
	if err != nil {
		g.log.Error("bad room nick", "room", r.jid, "err", err)
		return
	}

	join := outPresence{
		ID: uuid.NewString(),
		To: to.String(),
		MUC: &mucJoin{
			Password: r.cfg.Password,
			History:  &mucHistory{MaxStanzas: 0},
		},
	}

	g.log.Info("joining room", "room", r.jid.String(), "nick", r.cfg.Nick)
	if err := s.Encode(ctx, join); err != nil {
		g.log.Error("join room", "room", r.jid, "err", err)
		return
	}

	go func() {
		time.Sleep(joinTimeout)
		g.mu.Lock()
		defer g.mu.Unlock()
		if r.state == roomJoining && g.session == s {
			g.log.Warn("no subject received after join, assuming joined", "room", r.jid.String())
			r.state = roomJoined
		}
	}()
}

func (g *Gateway) rejoinLater(ctx context.Context, r *room, s *xmpp.Session) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(rejoinDelay):
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.session == s && r.state == roomLeft {
			g.joinLocked(ctx, s, r)
		}
	}()
}

// handle is called by Session.Serve for every incoming stanza. It must not use
// session send methods; replies go through t.
func (g *Gateway) handle(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
	// The start element is already consumed: put it back so the decoder
	// sees a balanced element.
	d := xml.NewTokenDecoder(xmlstream.MultiReader(xmlstream.Token(*start), t))

	switch start.Name.Local {
	case "message":
		var m inMessage
		if err := d.Decode(&m); err != nil && !errors.Is(err, io.EOF) {
			g.log.Warn("decode message", "err", err)
			return nil
		}
		g.onMessage(&m)
	case "presence":
		var p inPresence
		if err := d.Decode(&p); err != nil && !errors.Is(err, io.EOF) {
			g.log.Warn("decode presence", "err", err)
			return nil
		}
		g.onPresence(t, &p)
	case "iq":
		var iq inIQ
		if err := d.Decode(&iq); err != nil && !errors.Is(err, io.EOF) {
			g.log.Warn("decode iq", "err", err)
			return nil
		}
		return g.onIQ(t, &iq)
	}
	return nil
}

func (g *Gateway) onIQ(t xmlstream.TokenReadEncoder, iq *inIQ) error {
	switch {
	case iq.Type == stanza.GetIQ && iq.Version != nil:
		g.log.Info("version query", "from", iq.From.String())
		res := versionResult{ID: iq.ID, To: iq.From.String(), Type: string(stanza.ResultIQ)}
		res.Query.Name = g.cfg.IQ.Version.Name
		res.Query.Version = g.cfg.IQ.Version.Version
		res.Query.OS = g.cfg.IQ.Version.OS
		return t.Encode(res)
	case iq.Type == stanza.GetIQ && iq.Ping != nil,
		iq.Type == stanza.SetIQ && iq.Roster != nil: // roster push
		return t.Encode(emptyResult{ID: iq.ID, To: iq.From.String(), Type: string(stanza.ResultIQ)})
	}
	// Serve answers unhandled get/set IQs with service-unavailable.
	return nil
}

func (g *Gateway) onMessage(m *inMessage) {
	from := m.From

	g.mu.Lock()
	defer g.mu.Unlock()

	r := g.rooms[from.Bare().String()]

	switch m.Type {
	case stanza.ErrorMessage:
		g.log.Warn("message error", "from", from.String(), "id", m.ID)
		return

	case stanza.GroupChatMessage:
		if r == nil {
			return
		}
		nick := from.Resourcepart()

		if m.Subject != nil && m.Body == "" {
			if r.state == roomJoining {
				// The subject completes the join sequence.
				r.state = roomJoined
				g.log.Info("joined room", "room", r.jid.String(), "nick", r.myNick, "occupants", len(r.occupants))
				return
			}
			if m.Delay != nil || r.state != roomJoined {
				return
			}
			if nick == "" {
				nick = "<?>"
			}
			g.emit(bus.KindTopic, r.chatRef(), nick, *m.Subject, time.Time{})
			return
		}

		if m.Body == "" || m.Delay != nil || r.state != roomJoined || nick == r.myNick {
			// Our own messages are stored when they are sent.
			return
		}
		g.emit(bus.KindUser, r.chatRef(), nick, m.Body, time.Time{})

	default: // chat, normal
		if m.Body == "" {
			return
		}

		// Offline messages carry the time they were sent.
		var ts time.Time
		if m.Delay != nil {
			ts, _ = time.Parse(time.RFC3339, m.Delay.Stamp)
		}

		if r != nil {
			g.emit(bus.KindPrivMsg, r.chatRef(), from.Resourcepart(), m.Body, ts)
			return
		}

		bare := from.Bare()
		if bare.Equal(g.me.Bare()) {
			return
		}
		chat := bus.ChatRef{JID: bare.String(), Name: bare.Localpart(), IsMUC: false}
		g.emit(bus.KindUser, chat, bare.Localpart(), m.Body, ts)
	}
}

func (g *Gateway) onPresence(t xmlstream.TokenReadEncoder, p *inPresence) {
	from := p.From

	if p.Type == stanza.SubscribePresence {
		g.log.Info("subscription request", "from", from.Bare().String(), "auto_approve", g.cfg.Subscribes.AutoApprove)
		if g.cfg.Subscribes.AutoApprove {
			if err := t.Encode(outPresence{To: from.Bare().String(), Type: string(stanza.SubscribedPresence)}); err != nil {
				g.log.Error("approve subscription", "err", err)
			}
		}
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	r := g.rooms[from.Bare().String()]
	if r == nil {
		return
	}

	nick := from.Resourcepart()
	codes := p.MUCUser.codes()
	self := codes[110] || nick == r.myNick

	switch p.Type {
	case stanza.ErrorPresence:
		reason := ""
		if p.Error != nil {
			reason = p.Error.Inner
		}
		g.log.Error("room presence error", "room", r.jid.String(), "nick", nick, "error", reason)
		if self && r.state != roomJoined {
			r.state = roomLeft
			g.rejoinLater(context.Background(), r, g.session)
		}

	case stanza.UnavailablePresence:
		newNick := ""
		if p.MUCUser != nil && p.MUCUser.Item != nil {
			newNick = p.MUCUser.Item.Nick
		}

		if self {
			if codes[303] && newNick != "" {
				r.myNick = newNick
				return
			}
			g.log.Warn("left room", "room", r.jid.String(), "reason", leaveReason(codes))
			r.state = roomLeft
			g.rejoinLater(context.Background(), r, g.session)
			return
		}

		delete(r.occupants, nick)
		if r.state != roomJoined {
			return
		}
		text := leaveReason(codes)
		if codes[303] && newNick != "" {
			text = "renamed to " + newNick
		}
		g.emit(bus.KindLeave, r.chatRef(), nick, text, time.Time{})

	case "": // available
		if self {
			r.myNick = nick
			return
		}
		if _, known := r.occupants[nick]; known {
			return // status update
		}
		r.occupants[nick] = struct{}{}
		if r.state == roomJoined {
			g.emit(bus.KindJoin, r.chatRef(), nick, "", time.Time{})
		}
	}
}

func leaveReason(codes map[int]bool) string {
	switch {
	case codes[307]:
		return "KICKED"
	case codes[301]:
		return "BANNED"
	case codes[321]:
		return "AFFILIATION_CHANGE"
	case codes[322]:
		return "MODERATION_CHANGE"
	case codes[332]:
		return "SYSTEM_SHUTDOWN"
	}
	return "NORMAL"
}

func (r *room) chatRef() bus.ChatRef {
	return bus.ChatRef{JID: r.jid.String(), Name: r.jid.String(), IsMUC: true}
}

func (g *Gateway) emit(kind bus.Kind, chat bus.ChatRef, nick, text string, ts time.Time) {
	if ts.IsZero() {
		ts = time.Now()
	}
	g.events <- &bus.Event{
		ID:   uuid.NewString(),
		Time: ts.UTC(),
		Kind: kind,
		Chat: chat,
		Nick: nick,
		Text: text,
	}
}

// publishEvents forwards events to JetStream in order, retrying on failure so
// nothing is lost while NATS is briefly unavailable.
func (g *Gateway) publishEvents(ctx context.Context) {
	for {
		var ev *bus.Event
		select {
		case <-ctx.Done():
			return
		case ev = <-g.events:
		}

		for attempt := 0; ; attempt++ {
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := bus.Publish(pctx, g.js, ev.Subject(), ev.ID, ev)
			cancel()
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return
			}
			g.log.Error("publish event", "kind", ev.Kind, "err", err, "attempt", attempt)
			time.Sleep(min(time.Duration(attempt+1)*time.Second, 10*time.Second))
		}
	}
}

func (g *Gateway) handleCommand(m jetstream.Msg) {
	var cmd bus.SendCommand
	if err := json.Unmarshal(m.Data(), &cmd); err != nil {
		g.log.Error("bad send command, dropping", "err", err)
		_ = m.Term()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ev, err := g.send(ctx, &cmd)
	switch {
	case errors.Is(err, errNotConnected):
		_ = m.NakWithDelay(5 * time.Second)
		return
	case err != nil:
		g.log.Error("send message", "to", cmd.Chat.JID, "err", err)
		_ = m.NakWithDelay(5 * time.Second)
		return
	}

	g.events <- ev
	_ = m.Ack()
}

func (g *Gateway) send(ctx context.Context, cmd *bus.SendCommand) (*bus.Event, error) {
	to, err := jid.Parse(cmd.Chat.JID)
	if err != nil {
		return nil, fmt.Errorf("bad jid %q: %w", cmd.Chat.JID, err)
	}
	to = to.Bare()

	if cmd.ID == "" {
		cmd.ID = uuid.NewString()
	}

	ev := &bus.Event{
		ID:       cmd.ID,
		Time:     time.Now().UTC(),
		Kind:     bus.KindUser,
		Chat:     cmd.Chat,
		Text:     cmd.Text,
		Outgoing: true,
		Meta:     cmd.Meta,
	}
	msg := outMessage{ID: cmd.ID, Type: string(stanza.ChatMessage), Body: cmd.Text}

	g.mu.Lock()
	s := g.session
	switch {
	case cmd.Chat.IsMUC && cmd.PrivateNick != "":
		full, err := to.WithResource(cmd.PrivateNick)
		if err != nil {
			g.mu.Unlock()
			return nil, err
		}
		to = full
		ev.Kind = bus.KindPrivMsg
		ev.Nick = cmd.PrivateNick
	case cmd.Chat.IsMUC:
		msg.Type = string(stanza.GroupChatMessage)
		ev.Nick = g.me.Localpart()
		if r := g.rooms[to.String()]; r != nil {
			ev.Nick = r.myNick
		}
	default:
		ev.Nick = g.me.Localpart()
		if ev.Chat.Name == "" {
			ev.Chat.Name = to.Localpart()
		}
	}
	g.mu.Unlock()

	if s == nil {
		return nil, errNotConnected
	}

	msg.To = to.String()
	if err := s.Encode(ctx, msg); err != nil {
		return nil, err
	}
	return ev, nil
}
