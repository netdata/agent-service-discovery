FROM golang:alpine as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s" github.com/netdata/sd/cmd/sd

FROM alpine:3.12.0

COPY --from=builder /app/sd /app/

ENTRYPOINT ["/app/sd"]
