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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o /lwts ./server/cmd
# distroless nonroot = UID 65532
RUN mkdir -p /data && chown 65532:65532 /data

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=builder /lwts /lwts
COPY --from=frontend /app/dist /static
COPY --from=builder --chown=65532:65532 /data /data
VOLUME /data
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/lwts"]
