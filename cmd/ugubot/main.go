// Command ugubot runs one or all ugubot services.
//
//	ugubot [-config settings.toml] <xmpp-gateway|history|ai-worker|web-gateway|all>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"golang.org/x/sync/errgroup"

	"github.com/h5vx/ugubot/internal/ai"
	"github.com/h5vx/ugubot/internal/config"
	"github.com/h5vx/ugubot/internal/history"
	"github.com/h5vx/ugubot/internal/pg"
	"github.com/h5vx/ugubot/internal/web"
	"github.com/h5vx/ugubot/internal/xmppgw"
)

var services = []string{"history", "xmpp-gateway", "ai-worker", "web-gateway"}

func main() {
	var configFiles string
	flag.StringVar(&configFiles, "config", "settings.toml,.secrets.toml", "comma-separated config files")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: ugubot [-config files] <%s|all>\n", strings.Join(services, "|"))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load(strings.Split(configFiles, ",")...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	setupLogging(cfg.Log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, flag.Arg(0)); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, what string) error {
	names := []string{what}
	if what == "all" {
		names = services
	}

	if cfg.NATS.URL == "embedded" {
		if what != "all" {
			return errors.New(`nats.url = "embedded" works only with "ugubot all"`)
		}
		ns, err := startEmbeddedNATS(cfg.NATS.StoreDir)
		if err != nil {
			return err
		}
		defer ns.Shutdown()
		cfg.NATS.URL = ns.ClientURL()
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, name := range names {
		start, err := service(cfg, name)
		if err != nil {
			return err
		}
		g.Go(func() error {
			nc, err := connectNATS(cfg.NATS.URL, name)
			if err != nil {
				return err
			}
			defer nc.Drain()

			if err := start(ctx, nc); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		})
	}
	return g.Wait()
}

type pgPool = pgxpool.Pool

type startFunc func(context.Context, *nats.Conn) error

func service(cfg *config.Config, name string) (startFunc, error) {
	withDB := func(f func(context.Context, *nats.Conn, *pgPool) error) startFunc {
		return func(ctx context.Context, nc *nats.Conn) error {
			url, err := cfg.DatabaseURL()
			if err != nil {
				return err
			}
			pool, err := pg.Connect(ctx, url)
			if err != nil {
				return err
			}
			defer pool.Close()
			return f(ctx, nc, pool)
		}
	}

	switch name {
	case "history":
		return withDB(func(ctx context.Context, nc *nats.Conn, pool *pgPool) error {
			return history.Run(ctx, pool, nc)
		}), nil
	case "ai-worker":
		return withDB(func(ctx context.Context, nc *nats.Conn, pool *pgPool) error {
			return ai.Run(ctx, cfg.OpenAI, cfg.AdminJIDs, pool, nc)
		}), nil
	case "xmpp-gateway":
		return func(ctx context.Context, nc *nats.Conn) error {
			return xmppgw.Run(ctx, cfg.XMPP, nc)
		}, nil
	case "web-gateway":
		return func(ctx context.Context, nc *nats.Conn) error {
			return web.Run(ctx, cfg.WebUI, nc)
		}, nil
	}
	return nil, fmt.Errorf("unknown service %q; expected one of: %s, all", name, strings.Join(services, ", "))
}

func connectNATS(url, name string) (*nats.Conn, error) {
	return nats.Connect(url,
		nats.Name("ugubot-"+name),
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				slog.Warn("nats disconnected", "service", name, "err", err)
			}
		}),
		nats.ReconnectHandler(func(*nats.Conn) {
			slog.Info("nats reconnected", "service", name)
		}),
	)
}

func startEmbeddedNATS(storeDir string) (*natsserver.Server, error) {
	ns, err := natsserver.NewServer(&natsserver.Options{
		ServerName: "ugubot-embedded",
		Host:       "127.0.0.1",
		Port:       natsserver.RANDOM_PORT,
		JetStream:  true,
		StoreDir:   storeDir,
	})
	if err != nil {
		return nil, err
	}
	ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		return nil, errors.New("embedded NATS server did not start")
	}
	slog.Info("embedded NATS server started", "store_dir", storeDir)
	return ns, nil
}

func setupLogging(cfg config.Log) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if cfg.Format == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}
