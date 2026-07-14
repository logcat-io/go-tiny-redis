package server

import (
	"bufio"
	"fmt"
	"strings"
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
		if len(args) != 3 {
			return wrongArity(w, "set")
		}
		s.store.Set(args[1], args[2])
		return resp.WriteSimpleString(w, "OK")
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
	default:
		return resp.WriteError(w, fmt.Sprintf("ERR unknown command '%s'", cmd))
	}

}

func wrongArity(w *bufio.Writer, cmd string) error {
	return resp.WriteError(w, fmt.Sprintf("ERR wrong number of arguments for '%s' command", cmd))
}
