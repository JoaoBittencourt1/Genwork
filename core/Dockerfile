# Builds on the build machine's platform and cross-compiles, so multi-arch release images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/core/internal/command.Version=${VERSION}" -o /out/core ./cmd/core

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core /core
ENTRYPOINT ["/core"]
CMD ["serve"]
