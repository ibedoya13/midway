# Build stage
FROM golang:1.23-alpine AS build

WORKDIR /src

# Download the dependencies first so they are cached across code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary: Cloud Run runs this on a distroless base.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/midway ./cmd/midway

# Run stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/midway /midway

# Cloud Run injects PORT; the server reads it from the environment.
ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/midway"]
