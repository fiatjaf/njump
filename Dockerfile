# syntax=docker/dockerfile:1.4

#### Tailwind CSS build stage
FROM node:20 AS tailwindbuilder

# Set a temporary work directory
WORKDIR /app/tailwind

# Copy in the project files
COPY --link . .

# Install Tailwind CLI
RUN npm install tailwindcss

# Generate minified Tailwind CSS bundle
RUN npx tailwind -i base.css -o tailwind-bundle.min.css --minify

#### Go build stage
FROM golang:1.23.3-alpine AS gobuilder

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

# Build secp256k1
RUN git clone https://github.com/bitcoin-core/secp256k1.git && \
    cd secp256k1 && \
    ./autogen.sh && \
    ./configure --enable-module-extrakeys --enable-module-schnorrsig --prefix=$(pwd)/musl && \
    make install

# Build njump
RUN CGO_CFLAGS="-I$(pwd)/secp256k1/musl/include/" \
    CGO_LDFLAGS="-L$(pwd)/secp256k1/musl/lib" \
    GOOS=linux GOARCH=amd64 CC=$(which musl-gcc) \
    go build -tags libsecp256k1 \
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
COPY --from=gobuilder /app/relay-config.json.sample .

# Expose port
EXPOSE 2999

# Run njump
CMD ["./main"]
