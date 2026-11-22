FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/seed ./cmd/seed && CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && CGO_ENABLED=0 go build -o /out/scheduler ./cmd/scheduler && CGO_ENABLED=0 go build -o /out/demo ./cmd/demo && CGO_ENABLED=0 go build -o /out/delivery ./cmd/delivery && CGO_ENABLED=0 go build -o /out/grpcdemo ./cmd/grpcdemo && CGO_ENABLED=0 go build -o /out/events ./cmd/events && CGO_ENABLED=0 go build -o /out/loadcleanup ./cmd/loadcleanup
FROM alpine:3.23@sha256:469c9ea410b9215eaee7976ba6c0639039b444be8e5d638f7abbada189971eb2
WORKDIR /app
COPY --from=build /out/ ./
EXPOSE 8000
CMD ["./api"]
