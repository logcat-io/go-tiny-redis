# tinyredis (go-redis-lite)

Go로 만든 학습용 미니 Redis. TCP 위에서 RESP2 프로토콜을 직접 파싱하고,
TTL·스냅샷·graceful shutdown까지 구현했다. 실제 `redis-cli`, `redis-benchmark`로 접속/측정 가능.

## 실행

```sh
make run          # go run ./cmd/tinyredis (기본 :6379)
redis-cli -p 6379 # 다른 터미널에서 접속
```

플래그:

| 플래그 | 기본값 | 설명 |
|---|---|---|
| `-addr` | `:6379` | 리슨 주소 |
| `-snapshot` | `dump.trdb` | SAVE·부팅 로드에 쓰는 스냅샷 파일 (`""`면 비활성) |
| `-sweep-interval` | `1s` | 만료 키 능동 sweep 주기 |

## 지원 명령

`PING` · `SET key value [EX sec | PX ms]` · `GET` · `DEL` · `EXISTS` · `TTL` · `SAVE`

## 설계 포인트

- **RESP 파서**: 길이 선언을 신뢰하지 않는다 — 배열 64개 / bulk 512KB 상한 가드. 에러는 "다음 프레임을 신뢰할 수 있는가" 기준으로 3분류(클라이언트 잘못=연결 유지, 프로토콜 위반=연결 종료, I/O 에러=조용히 종료).
- **TTL**: 만료를 이벤트가 아닌 판정(`expired(now)`)으로. lazy 삭제(double-checked locking) + sweeper 병행. TTL 응답은 ceil — 산 키가 0을 답하지 않게.
- **스냅샷**: gob 인코딩, 같은 디렉토리 temp 파일 + `os.Rename`으로 원자적 교체. 만료 키는 저장·로드 양쪽에서 거른다.
- **graceful shutdown**: `signal.NotifyContext` → `ln.Close()`로 Accept 해제 → 3초 drain 후 남은 연결 강제 종료.

## 구조

```
cmd/tinyredis/   서버 엔트리포인트
cmd/loadtest/    연결 N개를 열고 유지하는 부하 도구 (C10K 관찰용)
cmd/racelab/     map 동시 쓰기 race 재현 실험
internal/resp/   RESP2 파싱/직렬화
internal/server/ accept 루프, 커맨드 dispatch, 종료 처리
internal/store/  RWMutex 보호 key-value 저장소, TTL, sweeper, 스냅샷
```

## 개발

```sh
make test   # go test ./...
make race   # -race 필수 — 동시성 코드는 race 없이 통과해도 못 믿는다
make vet
make bench  # 서버 띄운 뒤: redis-benchmark -t set,get -n 100000 -c 50
```

## 문서

- [docs/deep-dive.md](docs/deep-dive.md) — 커널 소켓·epoll/kqueue·netpoller·GMP·chan/mutex 인터널
- [docs/context-channel-shutdown.md](docs/context-channel-shutdown.md) — ctx/chan/select와 graceful shutdown 이해
- [docs/ch5-bench.md](docs/ch5-bench.md) — redis-benchmark + pprof 실측 기록
- [docs/load-test.md](docs/load-test.md) — 연결 유지 부하 측정
