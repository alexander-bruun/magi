# Use the official Golang image to create a build artifact.
FROM --platform=$BUILDPLATFORM golang:1.22.5 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Install `templ` using Go
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy the source code into the container
COPY . .

# Generate necessary files using `templ`
RUN TEMPL_EXPERIMENT=rawgo $GOPATH/bin/templ generate

# Build the Go app
ARG BUILDPLATFORM
ARG TARGETPLATFORM
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o magi .

# Start a new stage from scratch
FROM --platform=$BUILDPLATFORM alpine:3.20.2

# Install ca-certificates (required for HTTPS connections)
RUN apk --no-cache add ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/magi .

# Expose port 3000 to the outside world
EXPOSE 3000

# Command to run the executable
CMD ["./magi"]
