FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/throttler .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /out/throttler /app/throttler

EXPOSE 8080

ENTRYPOINT ["/app/throttler"]

CMD ["--addr=:8080", "--partitions=64", "--limit=10", "--window=1m"]