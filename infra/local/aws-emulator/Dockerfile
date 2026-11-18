# AWS Emulator for the local environment (TPL-0017): EventBridge, SQS, and the
# other AWS services that the contexts use, on one machine and without an
# account. AWS publishes a Linux package and no container image, so this file
# builds the image; compose.yaml runs it as the service `aws`.
#
#   docker build --tag aws-emulator infra/local/aws-emulator
#
# The package is pinned three ways (TPL-0024). The version names the file, the
# SHA-256 of each architecture shows that the file is the one that was locked,
# and rpm checks the signature of the package against the public key of AWS in
# signing-key.asc, so a new version cannot come from someone else.
#
# To update: change EMULATOR_VERSION and both checksums. The versions are in
# https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/versions.json;
# `curl --location <URL of the package> | shasum -a 256` gives a checksum. The
# tests of the adapters (`make test-integration`) and `make smoke` then show
# whether the new version still does what the template relies on.

FROM public.ecr.aws/amazonlinux/amazonlinux:2023-minimal@sha256:c64f3b683d01c1c874831adc6a7b5a2fd6f0f9affd58bc638939a9f46cc03543

ARG EMULATOR_VERSION=1.0.498
ARG EMULATOR_SHA256_ARM64=05d4cb0b37dc19d6580ad22fc472c92845e1ef36633b3a4a8eba987926fc4d69
ARG EMULATOR_SHA256_AMD64=df82916496908f4217a13e47609492fa3c53a53e9ff88041c58f53ef67dc8548
ARG TARGETARCH

LABEL org.opencontainers.image.title="aws-emulator" \
      org.opencontainers.image.description="AWS Emulator ${EMULATOR_VERSION} for the local environment" \
      org.opencontainers.image.source="https://github.com/ma-chisuke/go-nextjs-ddd-monorepo"

# The public key that signs the packages of AWS Emulator, from
# https://downloads.aws.com/keys/gpg/current.asc. Its fingerprint is
# 98BC F336 A3BE 53B7 34E9 67C5 2E5B F49B D940 F624.
COPY signing-key.asc /etc/aws-emulator/signing-key.asc

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# The home directory belongs to the user that runs the emulator, so that a
# volume mounted on its state directory gets the same owner.
RUN case "${TARGETARCH}" in \
      arm64) arch=aarch64 sha256="${EMULATOR_SHA256_ARM64}" ;; \
      amd64) arch=x86_64 sha256="${EMULATOR_SHA256_AMD64}" ;; \
      *) echo "AWS Emulator has no package for the architecture ${TARGETARCH}" >&2 && exit 1 ;; \
    esac \
    && curl --fail --silent --show-error --location --output /tmp/aws-emulator.rpm \
      "https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/aws-emulator-${EMULATOR_VERSION}-1-${arch}.rpm" \
    && echo "${sha256}  /tmp/aws-emulator.rpm" | sha256sum --check --quiet \
    && rpm --import /etc/aws-emulator/signing-key.asc \
    && rpm --checksig /tmp/aws-emulator.rpm | grep --quiet 'digests signatures OK$' \
    && rpm --install /tmp/aws-emulator.rpm \
    && rm /tmp/aws-emulator.rpm \
    && mkdir --parents /home/emulator/.aws/emulator \
    && chown --recursive 65532:65532 /home/emulator

# The user of the services of the contexts (backend/Dockerfile), by number.
# The emulator writes the certificate of its endpoint for its own user alone,
# and the services read it from a shared volume, so they must be that user.
USER 65532:65532

# No telemetry leaves the machine.
ENV HOME=/home/emulator \
    EMULATOR_TELEMETRY=off

EXPOSE 9297

# The endpoint is HTTPS with a certificate that the emulator makes at every
# start, for the names localhost and 127.0.0.1 only, and signs with the CA
# that it writes to its state directory.
HEALTHCHECK --interval=5s --timeout=3s --start-period=30s --start-interval=1s --retries=3 \
    CMD ["curl", "--fail", "--silent", "--output", "/dev/null", "--cacert", "/home/emulator/.aws/emulator/9297/ca.pem", "https://localhost:9297/__admin/health"]

# Every address, so that the other containers reach it. Requests are not
# signed: the emulator would make new credentials at every start, and nothing
# but this machine reaches the port (compose.yaml).
ENTRYPOINT ["aws-emulator", "--bind", "0.0.0.0", "--port", "9297", "--auth", "off"]
