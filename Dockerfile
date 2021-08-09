FROM alpine:3.14 AS builder

# update root certificates to copy into runtime image
RUN apk --no-cache add ca-certificates \
    && rm -rf /var/cache/apk/*

# download trivy
ARG TRIVY_VERSION=0.19.2
RUN wget -O- https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz | \
    tar -xzf - -C / \
    && /trivy --version

# download trivy database
RUN /trivy --cache-dir /trivy-cache image --light --no-progress --download-db-only

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /trivy /trivy
COPY --from=builder /trivy-cache /trivy-cache
COPY --from=builder /tmp /tmp
COPY estafette-extension-docker /

ENV PATH="/dod:$PATH" \
    ESTAFETTE_LOG_FORMAT="console" \
    DOCKER_BUILDKIT="1" \
    BUILDKIT_PROGRESS="plain"

ENTRYPOINT ["/estafette-extension-docker"]