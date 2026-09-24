FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/teendns ./cmd/teendns

FROM alpine:3.22
RUN addgroup -S teendns && adduser -S -G teendns teendns
COPY --from=builder /out/teendns /usr/local/bin/teendns
COPY --chown=teendns:teendns catalog /catalog
RUN chmod -R a+rX /catalog
USER teendns
ENTRYPOINT ["teendns"]
