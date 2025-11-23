# Guía de Pruebas - Implementación Raft

## 📋 Prerequisitos

- Go 1.19+ instalado
- Terminal con soporte para múltiples procesos en background

## 🚀 Ejecutar Pruebas

### Opción 1: Script Automático

```bash
cd services/db_service/cmd/raft_test
./test_raft.sh
```

### Opción 2: Manual (3 terminales)

**Terminal 1:**
```bash
cd services/db_service/cmd/raft_test
go run main.go node1
```

**Terminal 2:**
```bash
cd services/db_service/cmd/raft_test
go run main.go node2
```

**Terminal 3:**
```bash
cd services/db_service/cmd/raft_test
go run main.go node3
```

### Opción 3: Manual (Una terminal con tmux/screen)

```bash
cd services/db_service/cmd/raft_test

# Terminal 1
go run main.go node1 &

# Terminal 2 (nueva pestaña)
go run main.go node2 &

# Terminal 3 (nueva pestaña)
go run main.go node3 &
```

## ✅ Qué Verificar en los Logs

### 1. **Bug #6 Corregido**: No debe haber cientos de reseteos
```bash
# Antes (BUG): Cientos de líneas como:
# [node2] INFO: ... Temporizador de elección reseteado a ...

# Después (CORREGIDO): Solo reseteos normales cuando se reciben heartbeats
grep "Temporizador de elección reseteado" logs/node2.log | wc -l
# Debe ser un número razonable (< 50 para 15 segundos)
```

### 2. **Bug #4 Corregido**: Los comandos deben aplicarse
```bash
# Debe aparecer este mensaje cuando se propone un comando:
grep "Aplicando log" logs/*.log
# Debe mostrar: "[Nodo X] Aplicando log 1: Comando='SET x = 10'"
```

### 3. **Bug #2 Corregido**: commitIndex debe avanzar
```bash
# Debe aparecer este mensaje cuando se compromete un log:
grep "Avanzando commitIndex" logs/*.log
# Debe mostrar: "[Líder X] INFO: Avanzando commitIndex a 1"
```

### 4. **Elecciones Funcionando**: Un nodo debe convertirse en líder
```bash
# Debe aparecer:
grep "Transición a Líder" logs/*.log
# Debe mostrar: "[Nodo X] INFO: Transición a Líder."
```

### 5. **Replicación Funcionando**: Los logs deben replicarse
```bash
# Debe aparecer:
grep "Log replicado exitosamente" logs/*.log
# Debe mostrar mensajes en los seguidores
```

## 📊 Análisis de Logs

### Comandos útiles:

```bash
# Contar reseteos de timer (debe ser bajo)
grep -c "Temporizador de elección reseteado" logs/node2.log

# Ver elecciones
grep "Elección ganada\|Transición a Líder" logs/*.log

# Ver comandos aplicados
grep "Aplicando log" logs/*.log

# Ver commitIndex avanzando
grep "Avanzando commitIndex" logs/*.log

# Ver errores
grep "ERROR" logs/*.log
```

## 🎯 Resultados Esperados

### ✅ Comportamiento Correcto:
1. Un nodo se convierte en líder dentro de los primeros segundos
2. El líder envía heartbeats regularmente (cada 50ms)
3. Cuando node1 propone un comando (después de 5 segundos):
   - El comando se añade al log del líder
   - Se replica a los seguidores
   - Se compromete cuando la mayoría lo replica
   - Se aplica en todos los nodos (mensaje "Aplicando log")
4. No hay cientos de reseteos de timer
5. No hay race conditions visibles en los logs

### ❌ Si algo falla:
- Revisar los logs de ERROR
- Verificar que los puertos 8011, 8012, 8013 estén disponibles
- Verificar que no haya procesos anteriores corriendo

## 🔍 Debugging

Si algo no funciona:

1. **Verificar que los nodos se conecten:**
```bash
grep "Servidor RPC listo" logs/*.log
```

2. **Verificar elecciones:**
```bash
grep "Iniciando elección\|Elección ganada" logs/*.log
```

3. **Verificar heartbeats:**
```bash
grep "Enviando heartbeats" logs/*.log
```

4. **Verificar errores de conexión:**
```bash
grep "connection refused\|error al conectar" logs/*.log
```

## 📝 Notas

- Los nodos deben iniciarse casi simultáneamente (dentro de 1-2 segundos)
- El líder se elige aleatoriamente basado en los timeouts
- node1 intentará proponer un comando después de 5 segundos
- Los logs se guardan en `logs/node1.log`, `logs/node2.log`, `logs/node3.log`

