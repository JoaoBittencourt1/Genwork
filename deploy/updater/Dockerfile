# The updater (rule 84): the Docker CLI with its compose plugin, plus the genwork binary that runs the update engine.
# Build context is the repository root so the replace directive can reach sdk/go. The binary is cross-compiled on the
# build machine's platform, so multi-arch release images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY cli/go.mod cli/go.sum ./cli/
WORKDIR /src/cli
RUN go mod download
COPY cli ./
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/cli/internal/commands.Version=${VERSION}" -o /out/genwork ./cmd/genwork

FROM docker:29.9.2-cli
COPY --from=build /out/genwork /usr/local/bin/genwork
ENTRYPOINT []
CMD ["genwork", "updater"]
