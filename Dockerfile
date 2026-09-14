# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src

# Dependencies are copied separately so their layer is cached between source changes.
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker

FROM alpine:3.21 AS api
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/api /app/api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM alpine:3.21 AS worker
RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/worker /app/worker
USER app
ENTRYPOINT ["/app/worker"]
