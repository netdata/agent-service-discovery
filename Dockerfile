FROM golang:alpine as builder

WORKDIR /workspace
COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-w -s" github.com/netdata/sd/cmd/sd

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /workspace/sd /
ENTRYPOINT ["/sd"]
