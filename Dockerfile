FROM alpine:latest
WORKDIR /app
COPY go-raft ./
ENTRYPOINT ["./go-raft", "server"]