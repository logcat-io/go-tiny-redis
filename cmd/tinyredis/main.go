package main

import (
	"flag"
	"log/slog"
	"os"
	"tinyredis/internal/server"
)

func main() {
	addr := flag.String("addr", ":6379", "listen address")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	srv := server.New(*addr, logger)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server exited", "err", err)
		os.Exit(1)
	}
}
