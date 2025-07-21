FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and source files to ensure dependencies are resolved
COPY go.mod ./
COPY cmd ./cmd
RUN go mod tidy && go mod download

# Copy the rest of the application files
COPY . .
RUN apk add --no-cache gcc musl-dev # Required for CGO (go-sqlite3)
RUN CGO_ENABLED=1 GOOS=linux go build -o log-server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache sqlite

WORKDIR /app
COPY --from=builder /app/log-server .
COPY sql ./sql
COPY .env .

ENV DATABASE_PATH=/app/data/logdata.db
ENV PORT=8015
ENV CONTRACT_SECRET_KEYS='{"cont123":"secret123","cont456":"secret456"}'

VOLUME /app/data
EXPOSE 8015

# Initialize the database and start the server
CMD sh -c "mkdir -p /app/data && sqlite3 /app/data/logdata.db < /app/sql/init.sql && ./log-server"