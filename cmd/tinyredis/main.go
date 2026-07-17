package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	"tinyredis/internal/server"
	"tinyredis/internal/store"
)

func main() {
	addr := flag.String("addr", ":6379", "listen address")
	snapshot := flag.String("snapshot", "dump.trdb", "snapshot file path used by SAVE and boot load")
	sweepInterval := flag.Duration("sweep-interval", time.Second, "active expiry sweep interval")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	st := store.New()
	if *snapshot != "" {
		switch err := st.LoadFile(*snapshot); {
		case err == nil:
			logger.Info("snapshot loaded", "path", *snapshot)
		case errors.Is(err, fs.ErrNotExist):
			logger.Info("snapshot not found", "path", *snapshot)
		default:
			logger.Error("failed to load snapshot", "path", *snapshot, "err", err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go st.RunSweeper(ctx, *sweepInterval)

	srv := server.New(*addr, st, *snapshot, logger)
	if err := srv.ListenAndServe(ctx); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
	logger.Info("server exited")
}
