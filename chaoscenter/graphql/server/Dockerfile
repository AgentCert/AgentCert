# BUILD STAGE
FROM golang:1.24 AS builder

LABEL maintainer="LitmusChaos"

ARG TARGETOS=linux
ARG TARGETARCH

COPY . /gql-server
WORKDIR /gql-server/server

ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

RUN go env
RUN go mod download
RUN go mod tidy

# Clean existing generated files to force regeneration
RUN rm -rf graph/generated/ graph/model/models_gen.go graph/model/models.go 2>/dev/null || true

# Debug: Show what files we have
RUN echo "=== Current Directory ===" && pwd && ls -la

# Generate GraphQL code
RUN echo "=== Running go generate ===" && go generate ./... && echo "=== go generate completed ===" 

# Verify generated files exist and are non-empty
RUN echo "=== Checking generated files ===" \
    && ls -lh graph/generated/ \
    && test -s graph/generated/generated.go \
    && echo "Generated GraphQL code is present"

RUN CGO_ENABLED=0 go build -o /output/server -v

# DEPLOY STAGE
# Use Red Hat UBI minimal image as base
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

# Install helm CLI for agent deployments
RUN microdnf -y update \
    && microdnf -y install curl-minimal tar gzip \
    && curl -fsSL https://get.helm.sh/helm-v3.14.4-linux-amd64.tar.gz -o /tmp/helm.tar.gz \
    && tar -xzf /tmp/helm.tar.gz -C /tmp \
    && mv /tmp/linux-amd64/helm /usr/local/bin/helm \
    && chmod +x /usr/local/bin/helm \
    && rm -rf /tmp/helm.tar.gz /tmp/linux-amd64 \
    && curl -fsSL https://dl.k8s.io/release/v1.29.0/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl \
    && microdnf clean all

COPY --from=builder /output/server $APP_DIR/
COPY server/manifests/. $APP_DIR/manifests

RUN chown -R 65534:0 $APP_DIR/manifests && chmod -R 755 $APP_DIR/manifests
RUN chown 65534:0 $APP_DIR/server && chmod 755 $APP_DIR/server

WORKDIR $APP_DIR
USER 65534

CMD ["./server"]

EXPOSE 8080
