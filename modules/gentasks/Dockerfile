# Build context is the repository root so the replace directive can reach sdk/go.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY sdk/go ./sdk/go
COPY modules/gentasks/go.mod modules/gentasks/go.sum ./modules/gentasks/
WORKDIR /src/modules/gentasks
RUN go mod download
COPY modules/gentasks ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gentasks ./cmd/gentasks

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/gentasks /gentasks
ENTRYPOINT ["/gentasks"]
CMD ["serve"]
