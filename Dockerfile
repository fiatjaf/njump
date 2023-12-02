#### Tailwind CSS build stage
FROM node:20 as tailwindbuilder

# Set a temporary work directory
WORKDIR /app/tailwind

# Copy in the project files
COPY . .

# Install Tailwind CLI
RUN npm install tailwindcss

# Generate minified Tailwind CSS bundle
RUN npx tailwind -i tailwind.css -o tailwind-bundle.min.css --minify

#### Go build stage
FROM golang:1.21.4 as gobuilder

# Set a temporary work directory
WORKDIR /app

# Add necessary go files
COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Copy minified Tailwind CSS bundle
COPY --from=tailwindbuilder /app/tailwind/tailwind-bundle.min.css ./static/tailwind-bundle.min.css

# Build the go binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

#### Build final image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy Go binary
COPY --from=gobuilder /app/main .

# Run the application
CMD ["./main"]
