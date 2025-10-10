FROM golang:1.22-alpine

WORKDIR /app

# Instalar dependencias del sistema para SQLite
RUN apk add --no-cache gcc musl-dev sqlite

# Crear directorio para la BD
RUN mkdir -p /app/data && chmod 755 /app/data

# Copiar go.mod y go.sum primero para cache de dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN go build -o /events-service

# Puerto expuesto
EXPOSE 8002

# Comando de ejecución
CMD ["/events-service"]