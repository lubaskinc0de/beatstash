set dotenv-load := true

up:
    docker compose up postgres navidrome -d
    go run cmd/navidrome-tg/main.go
