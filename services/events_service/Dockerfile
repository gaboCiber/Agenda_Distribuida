# 3. Events Service (Agenda)
FROM golang:1.22-alpine

WORKDIR /app

# Copiar el código fuente
COPY . .

# Descargar dependencias y construir la aplicación
RUN go mod download && \
    go build -o /events-service

# Puerto en el que correrá el servicio
EXPOSE 8002

CMD ["/events-service"]
