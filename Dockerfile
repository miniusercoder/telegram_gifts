FROM ubuntu:14.04

# --- Установка зависимостей ---------------------------------------------------

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        wget \
        ca-certificates \
        git \
    && rm -rf /var/lib/apt/lists/*

# --- Установка Go ------------------------------------------------------------
ARG GOVERSION=1.24.2
RUN wget -q https://go.dev/dl/go${GOVERSION}.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go${GOVERSION}.linux-amd64.tar.gz && \
    rm go${GOVERSION}.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

# --- Установка UPX ------------------------------------------------------------
ARG UPXVERSION=5.0.2
RUN wget -q https://github.com/upx/upx/releases/download/v${UPXVERSION}/upx-${UPXVERSION}-amd64_linux.tar.xz && \
    tar -xf upx-${UPXVERSION}-amd64_linux.tar.xz && \
    mv upx-${UPXVERSION}-amd64_linux/upx /usr/local/bin/ && \
    rm -rf upx-${UPXVERSION}-amd64_linux*

# --- Рабочая директория ------------------------------------------------------
WORKDIR /build

# --- Загрузка зависимостей ----------------------------------------------------
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod \
    go mod download

# --- Копируем проект ----------------------------------------------------------
COPY . .

# --- Сборка ------------------------------------------------------------------
ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

RUN --mount=type=cache,target=/root/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -buildmode=c-shared \
    -o libtg.so \
    -trimpath \
    -buildvcs=false \
    -ldflags "-s -w -buildid="

# --- Сжатие бинарника --------------------------------------------------------
RUN chmod 755 libtg.so
RUN upx --best --lzma libtg.so

