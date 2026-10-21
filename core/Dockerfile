FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/JoaoBittencourt1/Genwork/core/internal/command.Version=${VERSION}" -o /out/core ./cmd/core

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core /core
ENTRYPOINT ["/core"]
CMD ["serve"]
