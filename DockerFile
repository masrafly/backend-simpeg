# --- TAHAP 1: BUILD BACKEND ---
FROM golang:1.21-alpine AS builder

# Install git dan bash untuk debugging
RUN apk add --no-cache git bash

WORKDIR /app

# Copy dependency Go
COPY go.mod go.sum ./
RUN go mod download

# Copy SEMUA file source code
COPY . .

# Cek apakah folder dist ada (DEBUGGING)
# Jika folder dist TIDAK ADA, proses akan berhenti di sini dan memberi error jelas
RUN if [ ! -d "./dist" ]; then echo "ERROR: Folder dist TIDAK DITEMUKAN! Pastikan sudah di-build dan di-push ke GitHub."; exit 1; fi

# Build aplikasi Go
RUN go build -o main .

# --- TAHAP 2: RUNNER (IMAGE RINGAN) ---
FROM alpine:latest

WORKDIR /root/

# Copy binary dari builder
COPY --from=builder /app/main .

# Copy folder dist dari builder
COPY --from=builder /app/dist ./dist

# Set Environment Variable agar Gin berjalan mode Release
ENV GIN_MODE=release

# Expose port 8080
EXPOSE 8080

# Jalankan aplikasi
CMD ["./main"]