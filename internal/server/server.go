package server

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"tinyredis/internal/resp"
	"tinyredis/internal/store"
)

type Server struct {
	addr  string
	log   *slog.Logger
	store *store.Store
}

func New(addr string, st *store.Store, log *slog.Logger) *Server {
	return &Server{addr: addr, store: st, log: log}
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.log.Info("listening on", "addr", ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Warn("accept failed", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			s.log.Warn("failed to close connection", "err", err)
		}
	}(conn)
	s.serveLoop(bufio.NewReader(conn), bufio.NewWriter(conn))
}

func (s *Server) serveLoop(reader *bufio.Reader, writer *bufio.Writer) {
	for {
		args, err := resp.ReadCommand(reader)
		if err != nil {
			if errors.Is(err, resp.ErrProtocol) {
				_ = resp.WriteError(writer, "ERR protocol error")
				_ = writer.Flush()
			}
			return
		}
		if err := s.dispatch(writer, args); err != nil {
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
	}
}
