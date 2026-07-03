package server

import (
	"bufio"
	"log/slog"
	"net"
	"strings"
)

type Server struct {
	addr string
	log  *slog.Logger
}

func New(addr string, log *slog.Logger) *Server {
	return &Server{addr: addr, log: log}
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
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		if _, err := w.WriteString(strings.ToUpper(line)); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}
	}
}
