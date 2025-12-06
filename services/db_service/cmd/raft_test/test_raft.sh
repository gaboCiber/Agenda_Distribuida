#!/bin/bash

# Script para probar la implementación de Raft
# Ejecuta 3 nodos en paralelo y los detiene después de un tiempo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Limpiar logs anteriores
echo "Limpiando logs anteriores..."
rm -rf logs/*.log

# Limpiar procesos anteriores si existen
echo "Deteniendo procesos anteriores..."
pkill -f "go run main.go" || true
pkill -f "raft_test" || true
sleep 1

echo "Iniciando nodos Raft..."
echo ""

# Iniciar los 3 nodos en background usando go run en grupos de procesos
cd "$SCRIPT_DIR"
setsid go run main.go node1 > /dev/null 2>&1 &
NODE1_PID=$!
echo "Iniciado node1 con PID $NODE1_PID"

setsid go run main.go node2 > /dev/null 2>&1 &
NODE2_PID=$!
echo "Iniciado node2 con PID $NODE2_PID"

setsid go run main.go node3 > /dev/null 2>&1 &
NODE3_PID=$!
echo "Iniciado node3 con PID $NODE3_PID"

echo "Nodos iniciados:"
echo "  - node1 (PID: $NODE1_PID)"
echo "  - node2 (PID: $NODE2_PID)"
echo "  - node3 (PID: $NODE3_PID)"
echo ""
echo "Esperando 15 segundos para que el sistema se estabilice..."
echo ""

# Esperar 15 segundos
sleep 15

echo "Deteniendo nodos..."
kill -- -$NODE1_PID 2>/dev/null || true
kill -- -$NODE2_PID 2>/dev/null || true
kill -- -$NODE3_PID 2>/dev/null || true

sleep 1

# Forzar kill si aún están corriendo
pkill -9 -f "go run main.go" 2>/dev/null || true
pkill -9 -f "raft_test" 2>/dev/null || true

echo ""
echo "Prueba completada. Revisa los logs en el directorio logs/"
echo ""
echo "Resumen de logs:"
echo "=================="
for log in logs/*.log; do
    if [ -f "$log" ]; then
        node=$(basename "$log" .log)
        lines=$(wc -l < "$log" 2>/dev/null || echo "0")
        echo "  $node: $lines líneas"
    fi
done
