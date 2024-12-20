# Base image
FROM golang:1.23.4

# Set work directory
WORKDIR /app

# Copy source code
COPY . .

# Install dependencies and build
RUN go mod tidy && go build -o app

# Expose the port
EXPOSE 8080

# Run the app
CMD ["./app"]
