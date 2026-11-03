# ==============================================================
# APEX FINTECH - PRODUCTION DOCKERFILE (MULTI-STAGE BUILD)
# ==============================================================

# STAGE 1: Build binary bằng Golang Alpine siêu nhẹ
FROM golang:alpine AS builder

WORKDIR /app

# Cài đặt git và ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Copy go module files và download dependencies
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /app/backend
RUN go mod download

# Copy toàn bộ mã nguồn
WORKDIR /app
COPY backend ./backend
COPY frontend ./frontend

# Build binary tối ưu (loại bỏ debug symbols để giảm dung lượng)
WORKDIR /app/backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/apex-server .

# STAGE 2: Image vận hành tối giản (chỉ chứa binary và static assets)
FROM alpine:latest

WORKDIR /app

# Cài đặt timezone Việt Nam và chứng chỉ SSL
RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Ho_Chi_Minh /etc/localtime \
    && echo "Asia/Ho_Chi_Minh" > /etc/timezone

# Copy binary từ builder stage
COPY --from=builder /app/apex-server /app/apex-server

# Copy tài nguyên tĩnh Frontend và Backend Static
COPY frontend /app/frontend
COPY backend/static /app/backend/static
COPY chucmung.html ladi.html tracker.js config.json /app/

# Expose cổng mặc định
EXPOSE 8080

# Health check định kỳ
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Khởi chạy ứng dụng
CMD ["/app/apex-server"]
