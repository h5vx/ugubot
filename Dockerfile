# One image for every service: `ugubot <service>` or `ugubot all`.

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN mkdir -p ../internal/web/dist && npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/web/dist/ internal/web/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ugubot ./cmd/ugubot

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 ugubot
COPY --from=build /ugubot /usr/local/bin/ugubot
USER ugubot
WORKDIR /app
EXPOSE 8000
ENTRYPOINT ["ugubot", "-config", "/app/settings.toml,/app/.secrets.toml"]
CMD ["all"]
