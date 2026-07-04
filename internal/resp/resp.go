package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	maxArrayLen = 64
	maxBulkLen  = 512 * 1024
)

var ErrProtocol = errors.New("resp: protocol error")

// ReadCommand 는 클라이언트 요청 한 건(*N 배열 + $ bulk string N개)을 읽는다.
//
//	*2\r\n$4\r\nPING\r\n$5\r\nhello\r\n  →  []string{"PING", "hello"}
func ReadCommand(r *bufio.Reader) ([]string, error) {
	line, err := readLine(r) // *2\r\n
	if err != nil {
		return nil, err
	}
	if len(line) == 0 || line[0] != '*' {
		return nil, fmt.Errorf("%w: expected array, got %q", ErrProtocol, line)
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil || n < 1 || n > maxArrayLen {
		return nil, fmt.Errorf("%w: invalid array length %d", ErrProtocol, n)
	}

	args := make([]string, n)
	for i := 0; i < n; i++ {
		s, err := readBulkString(r) // $4\r\n > PING\r\n > $5\r\n > hello\r\n
		if err != nil {
			return nil, err
		}
		args = append(args, s)
	}
	return args, nil
}

func readBulkString(r *bufio.Reader) (string, error) {
	line, err := readLine(r)
	if err != nil {
		return "", err
	}
	if len(line) == 0 || line[0] != '$' {
		return "", fmt.Errorf("%w: expected bulk string, got %q", ErrProtocol, line)
	}
	size, err := strconv.Atoi(line[1:])
	if err != nil || size < 0 || size > maxBulkLen {
		return "", fmt.Errorf("%w: invalid bulk string size %q", ErrProtocol, line[1:])
	}

	// 본문 + 후행 \r\n 까지 정확히 size + 2 바이트. \r\n(CRLF) 2바이트

	buf := make([]byte, size+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	if buf[size] != '\r' || buf[size+1] != '\n' {
		return "", fmt.Errorf("%w: bulk string not terminated by CRLF", ErrProtocol)
	}

	return string(buf[:size]), nil
}

// \r\n 으로 끝나는 한 줄을 읽고 \r\n 을 뗀 나머지를 돌려준다.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", fmt.Errorf("%w: line not terminated by CRLF", ErrProtocol)
	}
	return line[:len(line)-2], nil
}

func WriteSimpleString(w *bufio.Writer, s string) error {
	_, err := fmt.Fprintf(w, "+%s\r\n", s)
	return err
}

func WriteError(w *bufio.Writer, s string) error {
	_, err := fmt.Fprintf(w, "-%s\r\n", s)
	return err
}

func WriteBulkString(w *bufio.Writer, s string) error {
	_, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(s), s)
	return err
}

func WriteNullBulk(w *bufio.Writer) error {
	_, err := w.WriteString("$-1\r\n")
	return err
}

func WriteInteger(w *bufio.Writer, n int64) error {
	_, err := fmt.Fprintf(w, ":%d\r\n", n)
	return err
}
