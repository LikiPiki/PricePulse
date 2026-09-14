# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

# Dependencies are copied separately so their layer is cached between source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/worker ./cmd/worker

FROM scratch AS api
COPY --from=build /out/api /app/api
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM scratch AS worker
COPY --from=build /out/worker /app/worker
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 65532:65532
ENTRYPOINT ["/app/worker"]
