# Build context is the repository root so the replace directive can reach sdk/go.
# Builds on the build machine's platform and cross-compiles, so multi-arch release images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY modules/genmonitor/go.mod modules/genmonitor/go.sum ./modules/genmonitor/
WORKDIR /src/modules/genmonitor
RUN go mod download
COPY modules/genmonitor ./
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/modules/genmonitor/internal/command.Version=${VERSION}" -o /out/genmonitor ./cmd/genmonitor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/genmonitor /genmonitor
ENTRYPOINT ["/genmonitor"]
CMD ["serve"]
