# Use the official Golang image to create a build artifact.
FROM golang:1.22.5 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download -x

# Install `templ` using Go
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy the source code into the container
COPY . .

# Generate necessary files using `templ`
RUN TEMPL_EXPERIMENT=rawgo $GOPATH/bin/templ generate

# Build the Go app
RUN go build -o main .

# Start a new stage from scratch
FROM alpine:latest

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/main .

# Expose port 3000 to the outside world
EXPOSE 3000

# Command to run the executable
CMD ["./main"]
