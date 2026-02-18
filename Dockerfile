# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Stage 2: Build the dashboard
FROM node:20-alpine AS dashboard-builder

WORKDIR /app/dashboard

COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci

COPY dashboard/ ./
RUN npm run build

# Stage 3: Final image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=go-builder /server /app/server
COPY --from=dashboard-builder /app/dashboard/dist /app/dashboard/dist
COPY migrations/ /app/migrations/

EXPOSE 8080

CMD ["/app/server"]
