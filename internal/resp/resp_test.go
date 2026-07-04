package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"ping 단독", "*1\r\n$4\r\nPING\r\n", []string{"PING"}},
		{"set 3인자", "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n", []string{"SET", "foo", "bar"}},
		{"빈 bulk string", "*2\r\n$3\r\nGET\r\n$0\r\n\r\n", []string{"GET", ""}},
		{"본문에 개행 포함", "*2\r\n$3\r\nGET\r\n$4\r\na\r\nb\r\n", []string{"GET", "a\r\nb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadCommand(bufio.NewReader(strings.NewReader(tt.input)))
			if err != nil {
				t.Fatalf("ReadCommand() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TCP 는 스트림이라 한 요청이 바이트 단위로 쪼개져 도착할 수 있다.
// OneByteReader 가 그 최악 케이스를 재현한다 — io.ReadFull 대신 conn.Read 를 쓰면 이 테스트가 깨진다.
func TestReadCommand_fragmentedInput(t *testing.T) {
	input := "*3\r\n$3\r\nSET\r\n$5\r\nhello\r\n$5\r\nworld\r\n"
	r := bufio.NewReaderSize(iotest.OneByteReader(strings.NewReader(input)), 16)

	got, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("ReadCommand() error = %v", err)
	}
	want := []string{"SET", "hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadCommand() = %v, want %v", got, want)
	}
}

func TestReadCommand_protocolErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"배열 프리픽스 아님", "PING\r\n"},
		{"배열 길이가 숫자 아님", "*x\r\n"},
		{"배열 길이 0", "*0\r\n"},
		{"배열 길이 상한 초과", "*9999\r\n"},
		{"bulk 프리픽스 아님", "*1\r\n+PING\r\n"},
		{"bulk 길이 음수", "*1\r\n$-5\r\n"},
		{"CRLF 아닌 LF 종결", "*1\n$4\nPING\n"},
		{"bulk 본문 CRLF 미종결", "*1\r\n$4\r\nPINGXX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadCommand(bufio.NewReader(strings.NewReader(tt.input)))
			if !errors.Is(err, ErrProtocol) {
				t.Fatalf("ReadCommand() error = %v, want ErrProtocol", err)
			}
		})
	}
}

func TestReadCommand_eofIsNotProtocolError(t *testing.T) {
	_, err := ReadCommand(bufio.NewReader(strings.NewReader("")))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ReadCommand() error = %v, want io.EOF", err)
	}
	if errors.Is(err, ErrProtocol) {
		t.Fatalf("연결 종료(EOF)를 프로토콜 위반으로 오분류하면 안 된다")
	}
}

func TestWriters(t *testing.T) {
	tests := []struct {
		name  string
		write func(w *bufio.Writer) error
		want  string
	}{
		{"simple string", func(w *bufio.Writer) error { return WriteSimpleString(w, "OK") }, "+OK\r\n"},
		{"error", func(w *bufio.Writer) error { return WriteError(w, "ERR boom") }, "-ERR boom\r\n"},
		{"bulk string", func(w *bufio.Writer) error { return WriteBulkString(w, "foo") }, "$3\r\nfoo\r\n"},
		{"null bulk", WriteNullBulk, "$-1\r\n"},
		{"integer", func(w *bufio.Writer) error { return WriteInteger(w, -2) }, ":-2\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := bufio.NewWriter(&buf)
			if err := tt.write(w); err != nil {
				t.Fatalf("write error = %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("flush error = %v", err)
			}
			if buf.String() != tt.want {
				t.Fatalf("wrote %q, want %q", buf.String(), tt.want)
			}
		})
	}
}
