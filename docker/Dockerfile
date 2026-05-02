FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /snip ./cmd/api


FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /snip /snip

RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

CMD ["/snip"]
