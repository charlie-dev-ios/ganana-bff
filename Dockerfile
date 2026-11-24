# syntax=docker/dockerfile:1

# ビルドステージ: 静的リンクの単一バイナリを作る
FROM golang:1.26.3-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# 実行ステージ: シェルを持たない非 root の最小イメージ
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server

ENV GIN_MODE=release
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
