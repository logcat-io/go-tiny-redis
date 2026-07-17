package store

import (
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(s *Store) (advance func(d time.Duration)) {
	current := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return current }
	return func(d time.Duration) { current = current.Add(d) }
}

func TestSetGet(t *testing.T) {
	s := New()
	s.Set("hello", "world")

	got, ok := s.Get("hello")
	if !ok || got != "world" {
		t.Fatalf("Get() = (%q, %v), want (world, true)", got, ok)
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get(missing) = true, want false")
	}
}

func TestTTL_lazyExpiration(t *testing.T) {
	s := New()
	advance := fixedClock(s)

	s.SetWithTTL("session", "abc", 10*time.Second)

	if _, ok := s.Get("session"); !ok {
		t.Fatal("만료 전인데 Get 실패")
	}
	remaining, hasExpiry, exists := s.TTL("session")
	if !exists || !hasExpiry || remaining != 10*time.Second {
		t.Fatalf("TTL() = (%v, %v, %v), want (10s, true, true)", remaining, hasExpiry, exists)
	}

	advance(11 * time.Second)

	if _, ok := s.Get("session"); ok {
		t.Fatal("만료됐는데 Get 성공")
	}
	if s.Len() != 0 {
		t.Fatalf("lazy expire 후 Len() = %d, want 0 (Get 이 지워야 함)", s.Len())
	}
}

func TestTTL_semantics(t *testing.T) {
	s := New()
	fixedClock(s)

	s.Set("forever", "v")

	if _, _, exists := s.TTL("missing"); exists {
		t.Fatal("TTL() = (_, _, false), want (_, _, true)")
	}
	if _, hasExpiry, exists := s.TTL("forever"); !exists || hasExpiry {
		t.Fatal("TTL() = (_, true, true), want (_, false, true)")
	}
}

func TestSweeper_removesExpiredWithRead(t *testing.T) {
	s := New()
	advance := fixedClock(s)

	s.SetWithTTL("a", "1", time.Second)
	s.SetWithTTL("b", "2", time.Second)
	s.Set("kepp", "3")

	advance(2 * time.Second)

	if s.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", s.Len())
	}
	if removed := s.sweepOnce(); removed != 2 {
		t.Fatalf("sweepOnce() = %d, want 2", removed)
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestDelExists(t *testing.T) {
	s := New()
	advance := fixedClock(s)

	s.Set("a", "1")
	s.SetWithTTL("gone", "2", time.Second)
	advance(2 * time.Second)

	// EXISTS: 같은 키 중복 질의는 중복 카운트, 만료 키는 0 취급
	if n := s.Exists("a", "a", "gone", "missing"); n != 2 {
		t.Fatalf("Exists() = %d, want 2", n)
	}
	// DEL: 만료된 키는 지워도 카운트에 안 들어간다
	if n := s.Del("a", "gone", "missing"); n != 1 {
		t.Fatalf("Del() = %d, want 1", n)
	}
}

func TestSnapshot_roundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.trdb")

	src := New()
	advance := fixedClock(src)
	src.Set("plain", "v1")
	src.SetWithTTL("with-ttl", "v2", time.Hour)
	src.SetWithTTL("expired", "v3", time.Second)
	advance(2 * time.Second)

	if err := src.SaveFile(path); err != nil {
		t.Fatalf("SaveFile() error = %v", err)
	}

	dst := New()
	fixedClock(dst)
	if err := dst.LoadFile(path); err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if v, ok := dst.Get("plain"); !ok || v != "v1" {
		t.Fatalf("plain = (%q, %v), want (v1, true)", v, ok)
	}
	if v, ok := dst.Get("with-ttl"); !ok || v != "v2" {
		t.Fatalf("with-ttl = (%q, %v), want (v2, true)", v, ok)
	}
	if _, ok := dst.Get("expired"); ok {
		t.Fatal("만료 키가 스냅샷을 타고 부활했다")
	}
}

// go test -race 로 실행해야 의미가 있다 — 동시 접근 시 데이터 레이스가 없음을 검증.
func TestConcurrentAccess(t *testing.T) {
	s := New()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			s.Set("k", "v")
			s.Del("k")
		}
	}()
	for i := 0; i < 1000; i++ {
		s.Get("k")
		s.Exists("k")
	}
	<-done
}
