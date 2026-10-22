# The remote MCP server (design §12), reached only through the gateway at /mcp.
# Build context is the repository root so the replace directive can reach sdk/go. Builds on the build machine's
# platform and cross-compiles, so multi-arch release images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY mcp/go.mod mcp/go.sum ./mcp/
WORKDIR /src/mcp
RUN go mod download
COPY mcp ./
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/mcp/internal/command.Version=${VERSION}" -o /out/genwork-mcp ./cmd/genwork-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/genwork-mcp /genwork-mcp
ENTRYPOINT ["/genwork-mcp"]
CMD ["serve-http"]
