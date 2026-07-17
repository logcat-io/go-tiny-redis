package store

import (
	"context"
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type entry struct {
	value    string
	expireAt time.Time // IsZero: 만료 시각이 설정되지 않았다 -> TTL 제외
}

func (e entry) expired(now time.Time) bool {
	return !e.expireAt.IsZero() && now.After(e.expireAt)
}

type Store struct {
	mu   sync.RWMutex
	data map[string]entry
	now  func() time.Time
}

func New() *Store {
	return &Store{data: make(map[string]entry), now: time.Now}
}

func (s *Store) Set(key, value string) {
	s.SetWithTTL(key, value, 0)
}

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) {
	var expireAt time.Time
	if ttl > 0 {
		expireAt = s.now().Add(ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{value: value, expireAt: expireAt}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	// Go 의 sync.RWMutex 에는 승격 API 가 없다.
	// RLock 을 쥔 채 Lock 을 부르면 승격이 아니라 자기 자신이 read lock 을 풀 때까지 기다리는 데드락이다.
	// 유일한 길은 락을 완전히 놓았다가 다시 잡는 것이다.
	// 이때 tx의 판정 근거와 행동 대상이 다른 시점의 세계가 펼쳐질 수 있다.
	// 그래서 락을 잡은뒤에는 다시 값을 읽어야 한다. 이 패턴의 일반형이 double-checked locking 이다.
	// 싼 락으로 낙관적으로 확인 -> 비싼 락을 잡고 같은 조건을 다시 확인 -> 행동

	if !ok {
		return "", false
	}
	if !e.expired(s.now()) {
		return e.value, true
	}
	// RLock -> Lock 승격은 원자적이지 않다
	// 락을 다시 잡는 사이에 다른 고루팀이 키를 새로 SET 할 수 있기 때문에 반드시 재확인 후 지운다
	s.mu.Lock()
	if e2, ok2 := s.data[key]; ok2 && e2.expired(s.now()) {
		delete(s.data, key)
	}
	s.mu.Unlock()
	return "", false
}

// 삭제 된 것을 카운팅 해야한다
// 만료된 키는 다르게 취급
func (s *Store) Del(keys ...string) int {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for _, key := range keys {
		e, ok := s.data[key]
		if !ok {
			continue
		}
		delete(s.data, key)
		if !e.expired(now) {
			deleted++
		}
	}
	return deleted
}

func (s *Store) Exists(keys ...string) int {
	now := s.now()
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, key := range keys {
		if e, ok := s.data[key]; ok && !e.expired(now) {
			count++
		}
	}
	return count
}

func (s *Store) TTL(key string) (remaining time.Duration, hasExpiry bool, exists bool) {
	now := s.now()
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || e.expired(now) {
		return 0, false, false
	}
	if e.expireAt.IsZero() { // 만료 시각이 설정되지 않은 경우
		return 0, false, true
	}
	return e.expireAt.Sub(now), true, true
}

func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

// --- active expiration -----
// RunSweeper 는 interval 마다 만료 키를 능동 제거한다. ctx 취소로 내려간다.

func (s *Store) RunSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce()
		}
	}
}

func (s *Store) sweepOnce() int {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, e := range s.data {
		if e.expired(now) {
			delete(s.data, key)
			removed++
		}
	}

	return removed
}

// --- snapshot ---

type snapshotEntry struct {
	Value            string
	ExpireAtUnixNano int64
}

func (s *Store) SaveFile(path string) error {
	now := s.now()
	s.mu.Lock()
	dump := make(map[string]snapshotEntry, len(s.data))
	for key, e := range s.data {
		if e.expired(now) {
			continue
		}
		var nano int64
		if !e.expireAt.IsZero() {
			nano = e.expireAt.UnixNano()
		}
		dump[key] = snapshotEntry{Value: e.value, ExpireAtUnixNano: nano}
	}
	s.mu.Unlock()

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tinyredis-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // rename 성공 시엔 이미 없어서 no-op

	if err := gob.NewEncoder(tmp).Encode(dump); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *Store) LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var dump map[string]snapshotEntry
	if err := gob.NewDecoder(f).Decode(&dump); err != nil {
		return err
	}

	now := s.now()
	data := make(map[string]entry, len(dump))
	for key, se := range dump {
		e := entry{value: se.Value}
		if se.ExpireAtUnixNano != 0 {
			e.expireAt = time.Unix(0, se.ExpireAtUnixNano)
			if e.expired(now) {
				continue
			}
		}
		data[key] = e
	}

	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
	return nil
}
