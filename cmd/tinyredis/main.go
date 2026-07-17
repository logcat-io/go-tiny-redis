package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"
	"tinyredis/internal/server"
	"tinyredis/internal/store"
)

func main() {
	addr := flag.String("addr", ":6379", "listen address")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	st := store.New()
	srv := server.New(*addr, st, logger)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	go st.RunSweeper(ctx, time.Second)
}
