# Etapa de construcción
FROM golang:1.24.0-bullseye AS builder

# Instalar dependencias necesarias para compilación
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

# Establecer el directorio de trabajo
WORKDIR /app

# Copiar archivos de módulos primero
COPY go.mod go.sum ./

# Descargar y verificar dependencias
RUN go mod download && go mod verify

# Copiar el resto del código fuente
COPY . .

# Sincronizar dependencias y construir la aplicación
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /user-service ./cmd/user-service

# Etapa de ejecución
FROM debian:bullseye-slim

# Instalar dependencias en tiempo de ejecución
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Crear usuario no root
RUN groupadd -r appuser && useradd -r -g appuser appuser

# Establecer el directorio de trabajo
WORKDIR /app

# Copiar el binario desde la etapa de construcción
COPY --from=builder /user-service .

# Cambiar propietario de los archivos
RUN chown -R appuser:appuser /app

# Cambiar al usuario no root
USER appuser

# Puerto expuesto
EXPOSE 8080

# Comando para ejecutar la aplicación
CMD ["/app/user-service"]
