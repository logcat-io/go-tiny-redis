# answer/Makefile
# tinyredis — build / test / bench 표준 진입점
.PHONY: run build test vet race bench clean

run:
	go run ./cmd/tinyredis

build:
	go build -o bin/tinyredis ./cmd/tinyredis

test:
	go test ./...

# 동시성 코드는 -race 없이 통과해도 믿을 수 없다
race:
	go test -race ./...

vet:
	go vet ./...

# 서버를 먼저 띄운 뒤 실행: make run (다른 터미널에서)
bench:
	redis-benchmark -p 6379 -t set,get -n 100000 -c 50 -q

clean:
	rm -rf bin dump.trdb