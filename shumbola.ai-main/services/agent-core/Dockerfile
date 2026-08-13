FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/agent-core ./cmd/server

FROM alpine:3.20
# ttf-dejavu: Cyrillic-capable font for uz-Cyrl/ru PDF reports (TZ 6.5) —
# see internal/report.RenderPDF and cmd/server/main.go's detectReportFont.
RUN apk add --no-cache ttf-dejavu \
    && adduser -D -u 10001 shumbola \
    && mkdir -p /data \
    && chown shumbola:shumbola /data
COPY --from=build /out/agent-core /usr/local/bin/agent-core
USER shumbola
ENV LISTEN_ADDR=:8080
ENV AUDIT_LOG_PATH=/data/audit.log
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/agent-core"]
