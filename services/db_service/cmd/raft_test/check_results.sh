#!/bin/bash

# Script para verificar los resultados de las pruebas

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=========================================="
echo "Análisis de Resultados de Pruebas Raft"
echo "=========================================="
echo ""

# Verificar que existan logs
if [ ! -d "logs" ] || [ -z "$(ls -A logs/*.log 2>/dev/null)" ]; then
    echo "❌ No se encontraron logs. Ejecuta las pruebas primero."
    echo "   Directorio actual: $(pwd)"
    echo "   Buscando en: $SCRIPT_DIR/logs"
    exit 1
fi

LOG_DIR="$SCRIPT_DIR/logs"

echo "1. Verificando Bug #6 (Race condition en resetElectionTimer)..."
echo "   Contando reseteos de timer en node2:"
RESET_COUNT=$(grep -c "Temporizador de elección reseteado" "$LOG_DIR/node2.log" 2>/dev/null || echo "0")
if [ "$RESET_COUNT" -lt 100 ]; then
    echo "   ✅ CORRECTO: Solo $RESET_COUNT reseteos (esperado < 100)"
else
    echo "   ❌ PROBLEMA: $RESET_COUNT reseteos (demasiados, bug no corregido)"
fi
echo ""

echo "2. Verificando Bug #4 (Aplicación de logs)..."
APPLY_COUNT=$(grep -c "Aplicando log" "$LOG_DIR"/*.log 2>/dev/null || echo "0")
if [ "$APPLY_COUNT" -gt 0 ]; then
    echo "   ✅ CORRECTO: Se encontraron $APPLY_COUNT aplicaciones de log"
    echo "   Comandos aplicados:"
    grep "Aplicando log" "$LOG_DIR"/*.log 2>/dev/null | head -5
else
    echo "   ❌ PROBLEMA: No se encontraron aplicaciones de log (bug no corregido)"
fi
echo ""

echo "3. Verificando Bug #2 (commitIndex avanzando)..."
COMMIT_COUNT=$(grep -c "Avanzando commitIndex" "$LOG_DIR"/*.log 2>/dev/null || echo "0")
if [ "$COMMIT_COUNT" -gt 0 ]; then
    echo "   ✅ CORRECTO: commitIndex avanzó $COMMIT_COUNT veces"
    grep "Avanzando commitIndex" "$LOG_DIR"/*.log 2>/dev/null
else
    echo "   ⚠️  ADVERTENCIA: No se encontraron avances de commitIndex"
    echo "   (Puede ser normal si no se propusieron comandos o no se alcanzó mayoría)"
fi
echo ""

echo "4. Verificando elecciones..."
LEADER_COUNT=$(grep -c "Transición a Líder" "$LOG_DIR"/*.log 2>/dev/null || echo "0")
if [ "$LEADER_COUNT" -gt 0 ]; then
    echo "   ✅ CORRECTO: Se eligió un líder"
    grep "Transición a Líder" "$LOG_DIR"/*.log 2>/dev/null
else
    echo "   ❌ PROBLEMA: No se eligió ningún líder"
fi
echo ""

echo "5. Verificando replicación..."
REPLICATE_COUNT=$(grep -c "Log replicado exitosamente" "$LOG_DIR"/*.log 2>/dev/null || echo "0")
if [ "$REPLICATE_COUNT" -gt 0 ]; then
    echo "   ✅ CORRECTO: Se replicaron logs $REPLICATE_COUNT veces"
else
    echo "   ⚠️  ADVERTENCIA: No se encontraron replicaciones"
fi
echo ""

echo "6. Verificando errores..."
ERROR_COUNT=$(grep -c "ERROR" "$LOG_DIR"/*.log 2>/dev/null || echo "0")
if [ "$ERROR_COUNT" -eq 0 ]; then
    echo "   ✅ CORRECTO: No se encontraron errores"
else
    echo "   ⚠️  ADVERTENCIA: Se encontraron $ERROR_COUNT errores"
    echo "   Primeros errores:"
    grep "ERROR" "$LOG_DIR"/*.log 2>/dev/null | head -5
fi
echo ""

echo "=========================================="
echo "Resumen de Logs:"
echo "=========================================="
for log in "$LOG_DIR"/*.log; do
    if [ -f "$log" ]; then
        node=$(basename "$log" .log)
        lines=$(wc -l < "$log" 2>/dev/null || echo "0")
        echo "  $node: $lines líneas"
    fi
done
echo ""

