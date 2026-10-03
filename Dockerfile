FROM golang:1.27.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/beatstash ./cmd/beatstash


FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates ffmpeg

COPY --from=builder /app/bin/beatstash /app/beatstash

CMD ["/app/beatstash"]