FROM alpine:3.13 AS builder

ARG TRIVY_VERSION=0.16.0

RUN wget -O- https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz | \
    tar -xzf - -C / \
    && /trivy --version \
    && /trivy --cache-dir /trivy-cache image --light --no-progress --download-db-only \
    && /trivy --cache-dir /trivy-cache image --severity CRITICAL --light --skip-update --no-progress --ignore-unfixed alpine:3.11

FROM scratch

LABEL maintainer="estafette.io"

COPY ${ESTAFETTE_GIT_NAME} /
COPY ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /trivy /trivy
COPY --from=0 /trivy-cache /trivy-cache
COPY --from=0 /tmp /tmp

ENV PATH="/dod:$PATH" \
    ESTAFETTE_LOG_FORMAT="console" \
    BUILDKIT_PROGRESS="plain"

ENTRYPOINT ["/${ESTAFETTE_GIT_NAME}"]