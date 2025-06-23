# syntax=docker/dockerfile:1.4

#### Tailwind CSS build stage
FROM node:23-slim AS tailwindbuilder

# Set a temporary work directory
WORKDIR /app/tailwind

# Copy in the project files
COPY --link . .

# Install Tailwind CLI
RUN npm install tailwindcss

# Generate minified Tailwind CSS bundle
RUN npx tailwind -i base.css -o tailwind-bundle.min.css --minify

#### Go build stage
FROM golang:1.24.2-alpine AS gobuilder

# Add package
RUN apk add --no-cache autoconf automake libtool build-base musl-dev git

# Add necessary go files and download modules
WORKDIR /app
COPY --link . .
RUN go mod download

# Copy minified Tailwind CSS bundle
COPY --from=tailwindbuilder /app/tailwind/tailwind-bundle.min.css ./static/tailwind-bundle.min.css

# Generate Go codes from template files
RUN go get github.com/a-h/templ/runtime && \
    go run -mod=mod github.com/a-h/templ/cmd/templ generate

# Build njump
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=$(which musl-gcc) \
    go build -tags='libsecp256k1' \
    -ldflags="-s -w -linkmode external -extldflags '-static' -X main.compileTimeTs=$(date '+%s')" \
    -o main .

# Build go binary
FROM alpine:latest

# Add certificates
RUN apk --no-cache add ca-certificates

# Set work directory
WORKDIR /root

# Copy Go binary
COPY --from=gobuilder /app/main .

# Copy relay config
COPY --from=gobuilder /app/relay-config.json.sample relay-config.json

# Copy locale files for i18n
COPY --from=gobuilder /app/locales/ ./locales/

# Expose port
EXPOSE 2999

# Run njump
CMD ["./main"]
