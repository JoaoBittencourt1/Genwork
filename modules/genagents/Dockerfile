# Build context is the repository root so the replace directive can reach sdk/go.
# Builds on the build machine's platform and cross-compiles, so multi-arch release images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY modules/genagents/go.mod modules/genagents/go.sum ./modules/genagents/
WORKDIR /src/modules/genagents
RUN go mod download
COPY modules/genagents ./
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
RUN test -n "${VERSION}" || { echo "the VERSION build argument is required (rule 50)" >&2; exit 1; }
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/modules/genagents/internal/command.Version=${VERSION}" -o /out/genagents ./cmd/genagents

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/genagents /genagents
ENTRYPOINT ["/genagents"]
CMD ["serve"]
