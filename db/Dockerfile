# The container image that prepares the database of an environment at deploy
# time: one ECS task runs it before the services start, and
# infra/modules/database defines that task. db/prepare.sh says what it does.
# The build context is the root of the repository, because the image holds
# db/bootstrap.sql and the schema.sql of every context:
#
#   docker build --file db/Dockerfile --tag db .
#
# db/Dockerfile.dockerignore names what enters the build.
#
# The base is the image of PostgreSQL that compose.yaml runs, for its psql and
# bash; no server starts in it. pgschema is the release that mise.toml pins,
# checked against the SHA-256 that mise.lock records for Linux.
#
# Renovate updates the tag and the digest of the base image; keep them those
# of compose.yaml. When mise.toml moves pgschema, change its version and its
# checksums here: `make test-db-image` fails while the two differ.

ARG TARGETARCH

# pgschema for each platform of the image, in a stage named after the value of
# TARGETARCH for that platform. The stage pgschema is the one of the platform
# of the build, so a build downloads one binary only.
FROM scratch AS amd64
ADD --chmod=0755 --checksum=sha256:0f3915acdd15fcbe28e4aa14ad08a7e558a21669e8d1d5afa95ad590e59647f7 \
    https://github.com/pgplex/pgschema/releases/download/v1.14.1/pgschema-1.14.1-linux-amd64 /pgschema

FROM scratch AS arm64
ADD --chmod=0755 --checksum=sha256:6a1617a21ba786a60c763e764437498e58d22f779e5e1e88684c1af2ad066688 \
    https://github.com/pgplex/pgschema/releases/download/v1.14.1/pgschema-1.14.1-linux-arm64 /pgschema

FROM ${TARGETARCH} AS pgschema

FROM docker.io/library/postgres:18.7@sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328

LABEL org.opencontainers.image.title="db" \
      org.opencontainers.image.description="Prepares the database: roles, schemas, and the schema of every context" \
      org.opencontainers.image.source="https://github.com/ma-chisuke/go-nextjs-ddd-monorepo" \
      org.opencontainers.image.licenses="MIT-0"

COPY --from=pgschema /pgschema /usr/local/bin/pgschema

# The files at the paths of the repository, as db/prepare.sh expects them.
WORKDIR /app
COPY db/bootstrap.sql db/prepare.sh db/
COPY backend/internal/ backend/internal/

# The user postgres of the base image, by number, so that a runtime can see
# that it is not root without reading /etc/passwd. The script writes no file,
# so the root file system can be read-only.
USER 999:999

# The task runs once and stops; there is no server whose health to check.
HEALTHCHECK NONE

ENTRYPOINT ["/app/db/prepare.sh"]
CMD []
