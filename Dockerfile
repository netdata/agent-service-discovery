FROM golang:1.23.2-alpine as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o agent_sd github.com/netdata/sd/cmd/sd

FROM alpine:3.20.3

COPY --from=builder /app/agent_sd /app/

ENTRYPOINT ["/app/agent_sd"]
