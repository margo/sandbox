# Start from official Golang image
FROM golang:1.25.10

# Create working directory
WORKDIR /app

# Copy source code
COPY . .

# Download dependencies and build
RUN go mod tidy
RUN go build -o myapp .

# Run the app
CMD ["./myapp"]