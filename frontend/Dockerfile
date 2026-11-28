# The container image of the frontend, the Next.js app (TPL-0014). The build
# context is the root of the repository, because the frontend is a package of
# the pnpm workspace and installs from the lockfile at the root:
#
#   docker build --file frontend/Dockerfile --tag web .
#
# frontend/Dockerfile.dockerignore names what enters the build.
#
# The build stage installs the dependencies that pnpm-lock.yaml names, with
# the Node of mise.toml and the pnpm of package.json, and runs `next build`,
# whose standalone output is a server with only the files that it needs. The
# image holds that output and nothing else, on distroless Node, and runs it as
# a user other than root (TPL-0024).
#
# Renovate updates the tags and the digests of both images. The Node of both
# must be the Node of mise.toml, and the pnpm below the one that package.json
# names; pnpm refuses to install when it is another version.

FROM docker.io/library/node:26.13.1-slim@sha256:abe7e5a82b61a8ef67b315b85497c9e65173387ccf2f4a029248184b8e020be5 AS build

# pnpm, as the package that the registry of npm serves for this version,
# checked against its SHA-256. Node no longer ships Corepack.
ADD --checksum=sha256:0791469382b579869c51827759876616bc3eae5296d8fa880bbc3c9c1e8cf6f7 \
    https://registry.npmjs.org/pnpm/-/pnpm-12.21.0.tgz /tmp/pnpm.tgz
RUN npm install --global --no-audit --no-fund /tmp/pnpm.tgz \
    && rm /tmp/pnpm.tgz

# No usage data leaves the build.
ENV NEXT_TELEMETRY_DISABLED=1 \
    CI=true

WORKDIR /src

# The dependencies first, in a layer of their own, so that a change to the
# code does not install them again. Only the frontend and what it depends on:
# the tools of the root package (textlint, Redocly) stay out. The settings of
# pnpm-workspace.yaml apply here too, such as the refusal of build scripts.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json frontend/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile --filter frontend...

COPY frontend/ frontend/

# The standalone server serves the static files of the build only when they
# sit next to it.
WORKDIR /src/frontend
RUN node_modules/.bin/next build \
    && cp -r .next/static .next/standalone/frontend/.next/static

# distroless Node: the runtime, CA certificates, time zone data, and a user
# "nonroot"; no shell and no package manager.
FROM gcr.io/distroless/nodejs26-debian13:nonroot@sha256:a84242e747fbb5aaaa0180c273b6f7130ca34b3300082192ef58eea96089eb3c

LABEL org.opencontainers.image.title="web" \
      org.opencontainers.image.description="The Next.js frontend" \
      org.opencontainers.image.source="https://github.com/ma-chisuke/go-nextjs-ddd-monorepo" \
      org.opencontainers.image.licenses="MIT-0"

WORKDIR /app
COPY --from=build /src/frontend/.next/standalone ./
COPY frontend/healthcheck.mjs ./healthcheck.mjs

# Every address, so that the platform reaches the server, on the port below.
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    HOSTNAME=0.0.0.0 \
    PORT=3000

# The user "nonroot" of distroless, by number, so that a runtime can see that
# it is not root without reading /etc/passwd.
USER 65532:65532

EXPOSE 3000

# Node checks the health of the server, because the image has no shell or
# curl. ECS ignores this instruction; its task definition runs the same
# command.
HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --start-interval=1s --retries=3 \
    CMD ["/nodejs/bin/node", "/app/healthcheck.mjs"]

# The entrypoint of the base image is Node.
CMD ["frontend/server.js"]
