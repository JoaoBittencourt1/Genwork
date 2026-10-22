# Build context is the repository root so the replace directive can reach sdk/go.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY modules/genmonitor/go.mod modules/genmonitor/go.sum ./modules/genmonitor/
WORKDIR /src/modules/genmonitor
RUN go mod download
COPY modules/genmonitor ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/genmonitor ./cmd/genmonitor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/genmonitor /genmonitor
ENTRYPOINT ["/genmonitor"]
CMD ["serve"]
