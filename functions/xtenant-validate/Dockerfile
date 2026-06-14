# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS build

WORKDIR /fn

ENV CGO_ENABLED=0

RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=netrc,target=/root/.netrc \
    TOKEN=$(awk '/password/{print $2}' /root/.netrc) \
    && GIT_CONFIG_COUNT=1 \
       GIT_CONFIG_KEY_0="url.https://x-access-token:${TOKEN}@github.com/.insteadOf" \
       GIT_CONFIG_VALUE_0="https://github.com/" \
       GOPRIVATE=github.com/rezakaramad/* \
       go mod download

ARG TARGETOS
ARG TARGETARCH

RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /function .

FROM gcr.io/distroless/static-debian12:nonroot AS image
WORKDIR /
COPY --from=build /function /function
EXPOSE 9443
USER nonroot:nonroot
ENTRYPOINT ["/function"]
