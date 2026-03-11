FROM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -ldflags "-s -w" -o /pagefire ./cmd/pagefire

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /pagefire /usr/local/bin/pagefire

RUN useradd -r -s /bin/false pagefire
USER pagefire

EXPOSE 3000
ENTRYPOINT ["pagefire"]
CMD ["serve"]
