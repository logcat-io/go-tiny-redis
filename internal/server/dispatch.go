package server

import (
	"bufio"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"tinyredis/internal/resp"
)

/*

에러의 3분류

클라이언트 잘못 = -ERR 응답 후 연결 유지
프로토콜 위반 = -ERR protocol error 후 연결 종료
I/O 에러 = 조용히 종료

*/

func (s *Server) dispatch(w *bufio.Writer, args []string) error {
	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "PING":
		if len(args) == 2 {
			return resp.WriteBulkString(w, args[1])
		}
		return resp.WriteSimpleString(w, "PONG")
	case "SET":
		if len(args) != 3 && len(args) != 5 {
			return wrongArity(w, "set")
		}
		return s.handleSet(w, args)
	case "GET":
		if len(args) != 2 {
			return wrongArity(w, "get")
		}
		v, ok := s.store.Get(args[1])
		if !ok {
			return resp.WriteNullBulk(w)
		}
		return resp.WriteBulkString(w, v)
	case "DEL":
		if len(args) < 2 {
			return wrongArity(w, "del")
		}
		return resp.WriteInteger(w, int64(s.store.Del(args[1:]...)))
	case "EXISTS":
		if len(args) < 2 {
			return wrongArity(w, "exists")
		}
		return resp.WriteInteger(w, int64(s.store.Exists(args[1:]...)))
	case "TTL":
		if len(args) != 2 {
			return wrongArity(w, "ttl")
		}
		remaining, hasExpiry, exists := s.store.TTL(args[1])
		switch {
		case !exists:
			return resp.WriteInteger(w, -2) // redis 규약: 키 없음
		case !hasExpiry:
			return resp.WriteInteger(w, -1) // redis 규약: 만료 미설정
		default:
			return resp.WriteInteger(w, int64(math.Ceil(remaining.Seconds())))
		}
	default:
		return resp.WriteError(w, fmt.Sprintf("ERR unknown command '%s'", cmd))
	}

}

func (s *Server) handleSet(w *bufio.Writer, args []string) error {
	if len(args) != 3 && len(args) != 5 {
		return wrongArity(w, "set")
	}
	var ttl time.Duration
	if len(args) == 5 {
		n, err := strconv.ParseInt(args[4], 10, 64)
		if err != nil || n <= 0 {
			return resp.WriteError(w, "ERR invalid expire time in 'set' command")
		}
		switch strings.ToUpper(args[3]) {
		case "EX":
			ttl = time.Duration(n) * time.Second
		case "PX":
			ttl = time.Duration(n) * time.Millisecond
		default:
			return resp.WriteError(w, "ERR syntax error")
		}
	}
	s.store.SetWithTTL(args[1], args[2], ttl)
	return resp.WriteSimpleString(w, "OK")
}

func wrongArity(w *bufio.Writer, cmd string) error {
	return resp.WriteError(w, fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
}
