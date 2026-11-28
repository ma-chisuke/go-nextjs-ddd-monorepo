# The development container: the same environment for every contributor, and
# an isolated place where a coding agent can run on its own.
#
# The image holds the operating system, a user, and mise. The tools come from
# mise.toml and mise.lock, and `make setup` installs them when the container is
# created (postCreateCommand in devcontainer.json), so a tool update needs no
# new image. The tools and the caches live in volumes and survive a rebuild.
#
# node_modules and .venv in the workspace are built for Linux here. To go back
# to the host with the same checkout, run `make setup` there again.

# buildpack-deps has git, curl, make, and a C compiler for `go test -race`, so
# the image installs no packages of its own.
FROM docker.io/library/buildpack-deps:trixie@sha256:086433f3c4cca3435eba355f585f55d593751a35c865745d88ce97e7258e3fc9

# mise, pinned by version and by the checksums in SHASUMS256.txt of its release.
ARG MISE_VERSION=2026.11.9
ARG MISE_SHA256_LINUX_X64=f13f3e5ec88807e0161669b3aa07a91d84478cdf1e95b5b92b63f85a25d42395
ARG MISE_SHA256_LINUX_ARM64=b8dd05af6447cee933928664982a8cb2fa646e6951cfc8539db23ce2554a7b04

# The user "dev". devcontainer.json updates the IDs to match the host.
ARG USER_UID=1000
ARG USER_GID=1000

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN case "$(uname -m)" in \
      x86_64) arch=x64 checksum="${MISE_SHA256_LINUX_X64}" ;; \
      aarch64) arch=arm64 checksum="${MISE_SHA256_LINUX_ARM64}" ;; \
      *) echo "unsupported architecture: $(uname -m)" >&2 && exit 1 ;; \
    esac \
    && curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
      --output /usr/local/bin/mise \
      "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-${arch}" \
    && echo "${checksum}  /usr/local/bin/mise" | sha256sum --check --strict - \
    && chmod 0755 /usr/local/bin/mise \
    && groupadd --gid "${USER_GID}" dev \
    && useradd --uid "${USER_UID}" --gid "${USER_GID}" --create-home --shell /bin/bash dev \
    && mkdir -p /home/dev/.cache /home/dev/.local/share/mise /home/dev/.claude \
    && chown -R dev:dev /home/dev

USER ${USER_UID}:${USER_GID}

# The tools that mise installs, for interactive shells; make finds them itself.
# Every cache goes under ~/.cache, which is a volume.
ENV PATH=/home/dev/.local/share/mise/shims:${PATH} \
    GOMODCACHE=/home/dev/.cache/go-mod
