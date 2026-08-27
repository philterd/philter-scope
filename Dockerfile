# Carries both binaries: philterscope-serve hosts the evaluation UI,
# philterscope-audit scores redaction output against a golden dataset.
# The UI is embedded with go:embed, so nothing else is needed at runtime.

FROM golang:1.26-alpine AS build

WORKDIR /src

# Module files first, so dependency download caches separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Reported by the server's health endpoint; pass --build-arg VERSION=1.2.3.
ARG VERSION=dev
ENV LDFLAGS="-s -w -X github.com/philterd/philterscope/internal/server.Version=$VERSION"

RUN CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" \
        -o /out/philterscope-serve ./cmd/philterscope-serve \
 && CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" \
        -o /out/philterscope-audit ./cmd/philterscope-audit

FROM alpine:3.24

# ca-certificates is needed to reach Philter, MongoDB or Ollama over TLS. The
# upgrade picks up package fixes published after the base image was last built,
# which the push-image.sh vulnerability scan otherwise blocks a push on.
RUN apk upgrade --no-cache \
 && apk add --no-cache ca-certificates \
 && adduser -D -u 65532 philterscope

COPY --from=build /out/philterscope-serve /usr/local/bin/philterscope-serve
COPY --from=build /out/philterscope-audit /usr/local/bin/philterscope-audit

USER philterscope
WORKDIR /data

EXPOSE 5000

# CMD not ENTRYPOINT, so the audit binary can be selected by overriding it.
CMD ["philterscope-serve"]
