# syntax=docker/dockerfile:1

# --- Builder Stage -----------------------------------------------------------
FROM ubuntu:22.04 AS builder

# --- Установка зависимостей ---------------------------------------------------
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        wget \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# --- Установка Go ------------------------------------------------------------
ARG GOVERSION=1.25.6
RUN wget -q https://go.dev/dl/go${GOVERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GOVERSION}.linux-amd64.tar.gz && \
    rm go${GOVERSION}.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

# Рабочая директория внутри контейнера
WORKDIR /build

# Копируем файлы модулей для кеширования зависимостей
COPY go.mod go.sum ./

# Загрузка зависимостей с использованием кеша
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go mod download

# Копируем исходный код
COPY . .

# Настройка переменных окружения для сборки
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

# Сборка проекта с использованием кеша компиляции
RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -buildmode=c-shared \
    -o libtg.so \
    -buildvcs=false \
    -trimpath \
    -ldflags "-s -w -buildid="

# --- Final Stage -------------------------------------------------------------
FROM alpine

WORKDIR /dist

# Копируем только скомпилированную библиотеку из builder
COPY --from=builder /build/libtg.so .
COPY --from=builder /build/libtg.h .

# Образ готов к использованию (содержит только библиотеку)

