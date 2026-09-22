set dotenv-load := true

up:
    docker compose up -d
    go run cmd/navidrome-tg/main.go