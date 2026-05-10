ARG ALPINE_VERSION=3.23.4
ARG GO_ALPINE_VERSION=3.23
ARG GO_VERSION=1.26.2
ARG TAILWIND_VERSION=v4.2.4

# Stage 1: Build Tailwind CSS
FROM alpine:${ALPINE_VERSION} AS tailwind-builder
ARG TAILWIND_VERSION
WORKDIR /app
RUN apk add --no-cache curl libstdc++ libgcc
RUN case "$(uname -m)" in \
        x86_64) tailwind_arch="x64-musl" ;; \
        aarch64) tailwind_arch="arm64-musl" ;; \
        *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;; \
    esac \
    && curl -fsSLo tailwindcss \
        "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-${tailwind_arch}" \
    && chmod +x tailwindcss
COPY internal/server/assets ./internal/server/assets
COPY internal/server/templates ./internal/server/templates
COPY internal/server/static ./internal/server/static
RUN ./tailwindcss \
    -i ./internal/server/assets/tailwind.css \
    -o ./internal/server/static/styles.css \
    --minify

# Stage 2: Build Go Binary
FROM golang:${GO_VERSION}-alpine${GO_ALPINE_VERSION} AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy generated css into static dir
COPY --from=tailwind-builder /app/internal/server/static/styles.css ./internal/server/static/styles.css
RUN CGO_ENABLED=0 GOOS=linux go build -o openiplookup ./cmd/openiplookup

# Stage 3: Final Image
FROM alpine:${ALPINE_VERSION}
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/data \
    && chown -R app:app /app
COPY --from=go-builder --chown=app:app /app/openiplookup .
COPY --chown=app:app config.json .

VOLUME ["/app/data"]

# The app listens on port 8080 by default based on config.json
EXPOSE 8080

USER app
CMD ["./openiplookup"]
