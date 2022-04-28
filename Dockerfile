FROM alpine:3.15 AS builder

# update root certificates to copy into runtime image
RUN apk --no-cache add ca-certificates \
    && rm -rf /var/cache/apk/* \
    && which cat

# download trivy
ARG TRIVY_VERSION=0.26.0
RUN wget -O- https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz | \
    tar -xzf - -C / \
    && /trivy --version

# download trivy database
RUN /trivy --cache-dir /trivy-cache image --no-progress --download-db-only

# Downloading gcloud package
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz

# Installing the package
RUN mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh

# Adding the package path to local
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /trivy /trivy
COPY --from=builder /trivy-cache /trivy-cache
COPY --from=builder /usr/local/gcloud/google-cloud-sdk /usr/local/gcloud/google-cloud-sdk
COPY --from=builder /tmp /tmp
COPY estafette-extension-docker /

ENV PATH="/dod:$PATH;$PATH:/usr/local/gcloud/google-cloud-sdk/bin" \
    ESTAFETTE_LOG_FORMAT="console" \
    DOCKER_BUILDKIT="1" \
    BUILDKIT_PROGRESS="plain" \
    GOOGLE_APPLICATION_CREDENTIALS="/key-file.json"

ENTRYPOINT ["/estafette-extension-docker"]