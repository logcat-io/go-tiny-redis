package store

import "testing"

func TestSetGetDelExists(t *testing.T) {
	s := New()
	s.Set("hello", "world")

	if got, ok := s.Get("hello"); !ok || got != "world" {
		t.Fatalf("Get() = (%q, %v), want (world, true)", got, ok)
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get(missing) = true, want false")
	}
	if n := s.Exists("hello", "hello", "missing"); n != 2 {
		t.Fatalf("Exists() = %d, want 2", n)
	}
	if n := s.Del("hello", "missing"); n != 1 {
		t.Fatalf("Del() = %d, want 1", n)
	}
	if s.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", s.Len())
	}
}

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
