# Stage 1: Frontend build (Vite)
FROM node:20-alpine AS frontend
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Go build
FROM golang:1.24-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY server/ server/
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o /lwts ./server/cmd

# Stage 3: Runtime
FROM alpine:3.21
LABEL org.opencontainers.image.title="LWTS" \
      org.opencontainers.image.description="Lightweight Task System — kanban board" \
      org.opencontainers.image.url="https://github.com/oceanplexian/lwts" \
      org.opencontainers.image.source="https://github.com/oceanplexian/lwts" \
      org.opencontainers.image.vendor="oceanplexian" \
      org.opencontainers.image.licenses="MIT"
RUN apk add --no-cache su-exec && adduser -D -u 65532 lwts
COPY --from=builder /lwts /lwts
COPY --from=frontend /app/dist /static
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
