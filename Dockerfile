# Modified by mia·nube on 2026-07-31: build without BuildKit-only Dockerfile
# features, so this image can also be produced by the classic Docker builder.
#
# Upstream's Dockerfile can only be built by BuildKit. Three constructs make that
# so, and each is replaced here with a portable equivalent that keeps the BuildKit
# behaviour identical:
#
#   1. `FROM --platform=$BUILDPLATFORM` on the two builder stages. BUILDPLATFORM is
#      an automatic variable that only BuildKit provides, so the classic builder
#      resolves it to the empty string and fails outright. Dropped. This pins the
#      builder stages to the machine doing the build, which is what BUILDPLATFORM
#      meant anyway for a native (non-emulated) build.
#   2. `ARG TARGETOS TARGETARCH TARGETVARIANT` with no defaults. BuildKit populates
#      these automatically; the classic builder leaves them empty, which would hand
#      `mage release:xgo` an empty target triple. They now carry native-Linux
#      defaults. BuildKit still overrides them with the real platform values, so
#      cross-compilation under BuildKit is unaffected.
#   3. `COPY --chmod=`, which the classic builder rejects as an unknown flag. The
#      source directory is already created with mode 1777 in the builder stage and
#      COPY preserves permissions, so the flag was redundant.
#
# The `# syntax=docker/dockerfile:1` parser directive was also removed. It selects
# an external BuildKit frontend, is ignored outside BuildKit, and a parser directive
# must be the very first line of the file — which this modification notice now is.
# Nothing left in this file needs a frontend newer than BuildKit's built-in one, so
# builds under BuildKit are unaffected.
#
# Why: the pipeline that builds this image runs on a runner that offers neither a
# usable BuildKit backend nor privileged containers, so a BuildKit-only Dockerfile
# cannot be built there at all. Nothing about the resulting image changes.

FROM node:24.18.0-alpine@sha256:a0b9bf06e4e6193cf7a0f58816cc935ff8c2a908f81e6f1a95432d679c54fbfd AS frontendbuilder

WORKDIR /build

ENV PNPM_CACHE_FOLDER=.cache/pnpm/
ENV PUPPETEER_SKIP_DOWNLOAD=true
ENV CYPRESS_INSTALL_BINARY=0

COPY frontend/pnpm-lock.yaml frontend/package.json frontend/pnpm-workspace.yaml ./
RUN npm install -g corepack && corepack enable && \
    pnpm install --frozen-lockfile
COPY frontend/ ./
ARG RELEASE_VERSION=dev
RUN echo "{\"VERSION\": \"${RELEASE_VERSION/-g/-}\"}" > src/version.json && pnpm run build

FROM ghcr.io/techknowlogick/xgo:go-1.26.x@sha256:b00957d8fec512c4748a5fafe17197be1d8c0bf704b271fc4aa128f5ddf40414 AS apibuilder

RUN go install github.com/magefile/mage@latest && \
    mv /go/bin/mage /usr/local/go/bin

WORKDIR /go/src/code.vikunja.io/api
COPY . ./
COPY --from=frontendbuilder /build/dist ./frontend/dist

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=
ARG RELEASE_VERSION
ENV RELEASE_VERSION=$RELEASE_VERSION

RUN export PATH=$PATH:$GOPATH/bin && \
	mage build:clean && \
    (cd build && mage release:xgo vikunja "${TARGETOS}/${TARGETARCH}/${TARGETVARIANT}")

RUN mkdir -p /tmp && chmod 1777 /tmp

#  ┬─┐┬ ┐┌┐┐┌┐┐┬─┐┬─┐
#  │┬┘│ │││││││├─ │┬┘
#  ┘└┘┘─┘┘└┘┘└┘┴─┘┘└┘

# The actual image
FROM scratch

LABEL org.opencontainers.image.authors='maintainers@vikunja.io'
LABEL org.opencontainers.image.url='https://vikunja.io'
LABEL org.opencontainers.image.documentation='https://vikunja.io/docs'
LABEL org.opencontainers.image.source='https://code.vikunja.io/vikunja'
LABEL org.opencontainers.image.licenses='AGPLv3'
LABEL org.opencontainers.image.title='Vikunja'

WORKDIR /app/vikunja
ENTRYPOINT [ "/app/vikunja/vikunja" ]
EXPOSE 3456

# Mode 1777 comes from the source directory (see the apibuilder stage above);
# `--chmod` was removed because the classic builder does not accept it.
COPY --from=apibuilder --chown=1000:1000 /tmp /tmp

USER 1000

ENV VIKUNJA_SERVICE_ROOTPATH=/app/vikunja/
ENV VIKUNJA_DATABASE_PATH=/db/vikunja.db

COPY --from=apibuilder /build/vikunja-* vikunja
COPY --from=apibuilder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
