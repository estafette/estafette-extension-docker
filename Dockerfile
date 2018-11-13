FROM docker:18.09.0

LABEL maintainer="estafette.io" \
      description="The estafette-extension-docker component is an Estafette extension to build. push and tag a Docker image"

COPY estafette-extension-docker /

ENTRYPOINT ["/estafette-extension-docker"]