# 임시
.PHONY: run build test vet

run:
	go run ./cmd/tinyredis

build:
	go build -o bin/tinyredis ./cmd/tinyredis

test:
	go test ./...

vet:
	go vet ./...