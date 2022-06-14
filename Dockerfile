FROM alpine:latest
RUN apk add --no-cache bash
WORKDIR /app
COPY go-raft ./
ENTRYPOINT ["./go-raft", "server"]