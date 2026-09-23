FROM node:22-alpine AS css

WORKDIR /src

RUN corepack enable && corepack prepare yarn@1.22.22 --activate

COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

COPY resources ./resources
COPY internal/adapter/html/renderer.go ./internal/adapter/html/renderer.go
RUN yarn build:css

FROM golang:1.27.1-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=css /src/resources/css/tailwindcss.css ./resources/css/tailwindcss.css

RUN CGO_ENABLED=1 go build -o /out/kwiki ./cmd/kwiki

FROM alpine:3.24

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 65532 kwiki \
    && mkdir -p /var/lib/data \
    && chown kwiki:kwiki /var/lib/data

WORKDIR /var/lib

COPY --from=builder /out/kwiki /usr/local/bin/kwiki

USER kwiki

EXPOSE 8000

VOLUME ["/var/lib/data"]

ENTRYPOINT ["kwiki"]
CMD ["-addr", ":8000"]
