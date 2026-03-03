# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o app .

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /build/app .
COPY --from=builder /build/static ./static

RUN mkdir -p /app/data

EXPOSE 8080

ENV TZ=Asia/Shanghai
ENV DB_PATH=/app/data/translations.db

CMD ["./app"]
