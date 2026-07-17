package server

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
	"tinyredis/internal/resp"
	"tinyredis/internal/store"
)

const drainTimeout = 3 * time.Second

type Server struct {
	addr         string
	store        *store.Store
	snapshotPath string
	log          *slog.Logger

	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func New(addr string, st *store.Store, snapshotPath string, log *slog.Logger) *Server {
	return &Server{addr: addr, store: st, snapshotPath: snapshotPath, log: log, conns: make(map[net.Conn]struct{})}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.log.Info("listening on", "addr", ln.Addr().String())

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	var wg sync.WaitGroup
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			s.log.Warn("accept failed", "err", err)
			continue
		}
		s.track(conn, true)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.track(conn, false)
			s.handleConn(conn)
		}()
	}

	s.log.Info("draining connections", "timeout", drainTimeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(drainTimeout):
		s.log.Warn("timed out waiting for connections to drain", "remaining", s.connCount())
		s.closeAll()
		<-done
	}
	return nil
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		args, err := resp.ReadCommand(r)
		if err != nil {
			if errors.Is(err, resp.ErrProtocol) {
				_ = resp.WriteError(w, "ERR protocol error")
				_ = w.Flush()
			}
			return
		}
		if err := s.dispatch(w, args); err != nil {
			s.log.Warn("write failed", "remote", conn.RemoteAddr().String(), "err", err)
			return
		}
		if err := w.Flush(); err != nil {
			return
		}
	}
}

func (s *Server) track(conn net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
}

func (s *Server) connCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

func (s *Server) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		conn.Close()
	}
}
