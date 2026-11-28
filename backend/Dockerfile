# The container image of one bounded context of the backend (TPL-0014). One
# Dockerfile builds every context: the build argument CONTEXT names the
# command in backend/cmd/<context>/, and the build context is backend/.
#
#   docker build --build-arg CONTEXT=facility --tag facility backend
#
# compose.yaml builds the image of each context this way, so a new context
# needs a service in compose.yaml and no Dockerfile of its own.
#
# The build stage compiles a static binary with the Go that mise.toml pins.
# The image holds that binary and nothing else, on distroless static, and runs
# it as a user other than root (TPL-0024).
#
# Renovate updates the tags and the digests of both images. The Go of the
# build image must be the Go that go.mod names, whose go line moves with
# mise.toml; the build fails when the two differ.

# The build runs on the platform of the builder and compiles for the platform
# of the image, so an image for another architecture needs no emulator.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build

# A static binary, built from the modules that go.sum names, with the Go of
# this image and no other.
ENV CGO_ENABLED=0 \
    GOFLAGS=-mod=readonly \
    GOTOOLCHAIN=local

WORKDIR /src

# The modules first, in a layer of their own, so that a change to the code
# does not download them again. `go mod download` checks every module against
# go.sum, and `go mod verify` checks the module cache once more.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    have="$(go env GOVERSION)" want="go$(go list -m -f '{{.GoVersion}}')" \
    && if [ "${have}" != "${want}" ]; then \
      echo "the build image has ${have}, but go.mod names ${want}: update the golang image in backend/Dockerfile together with mise.toml" >&2 \
      && exit 1; \
    fi \
    && go mod download \
    && go mod verify

COPY . .

# -trimpath and -buildvcs=false keep the paths of this machine and the state
# of the checkout out of the binary, and Go writes no build time, so the same
# source builds the same binary. -s -w drop the symbol table and the DWARF
# data; the module versions that scanners read stay in the binary.
ARG CONTEXT
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ ! -f "cmd/${CONTEXT}/main.go" ]; then \
      echo "set the build argument CONTEXT to a directory of backend/cmd/, such as facility" >&2 \
      && exit 1; \
    fi \
    && GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
      go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/service "./cmd/${CONTEXT}"

# distroless static: CA certificates, time zone data, and a user "nonroot";
# no shell and no package manager.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5

ARG CONTEXT
LABEL org.opencontainers.image.title="${CONTEXT}" \
      org.opencontainers.image.description="The ${CONTEXT} context of the backend" \
      org.opencontainers.image.source="https://github.com/ma-chisuke/go-nextjs-ddd-monorepo" \
      org.opencontainers.image.licenses="MIT-0"

COPY --from=build /out/service /service

# The user "nonroot" of distroless, by number, so that a runtime can see that
# it is not root without reading /etc/passwd.
USER 65532:65532

# HTTP_ADDR, unless the environment sets another port.
EXPOSE 8080

# The binary checks its own health (httpserver.Main), because the image has no
# shell or curl. ECS ignores this instruction; its task definition runs the
# same command.
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --start-interval=1s --retries=3 \
    CMD ["/service", "healthcheck"]

ENTRYPOINT ["/service"]
