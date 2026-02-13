# BUILD STAGE
FROM golang:1.24 AS builder

ARG TARGETOS=linux
ARG TARGETARCH

ADD . /auth-server
WORKDIR /auth-server

ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

RUN go env

RUN CGO_ENABLED=0 go build -o /output/server -v ./api/

# PACKAGING STAGE
# Use RedHat UBI minimal image as base
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6

LABEL maintainer="LitmusChaos"

# Langfuse Configuration (passed via --build-arg)
ARG LANGFUSE_HOST=""
ARG LANGFUSE_PUBLIC_KEY=""
ARG LANGFUSE_SECRET_KEY=""
ARG LANGFUSE_ORG_ID=""
ARG LANGFUSE_PROJECT_ID=""

ENV APP_DIR="/litmus"
ENV LANGFUSE_HOST=${LANGFUSE_HOST}
ENV LANGFUSE_PUBLIC_KEY=${LANGFUSE_PUBLIC_KEY}
ENV LANGFUSE_SECRET_KEY=${LANGFUSE_SECRET_KEY}
ENV LANGFUSE_ORG_ID=${LANGFUSE_ORG_ID}
ENV LANGFUSE_PROJECT_ID=${LANGFUSE_PROJECT_ID}

# Ensure base packages (including libxslt) are patched
RUN microdnf -y update && microdnf clean all

COPY --from=builder /output/server $APP_DIR/
RUN chown 65534:0 $APP_DIR/server && chmod 755 $APP_DIR/server

WORKDIR $APP_DIR
USER 65534

CMD ["./server"]

EXPOSE 3000
