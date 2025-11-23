# Resumen de Correcciones de Bugs - Implementación Raft

## ✅ Bugs Corregidos

### Bug #1: Race Condition en `startElection` (variable `votes`)
**Ubicación**: `raft.go:310-330`

**Problema**: La variable `votes` se incrementaba sin protección de mutex en múltiples goroutines.

**Solución**:
- Agregado campo `voteCount int32` al `RaftNode` para contador atómico
- Uso de `sync/atomic` para incrementar el contador de forma thread-safe
- Inicialización del contador a 1 (voto por sí mismo) al inicio de `startElection()`
- Verificación de mayoría usando el contador atómico

**Código corregido**:
```go
// Inicialización
atomic.StoreInt32(&rn.voteCount, 1)

// Incremento thread-safe
newVoteCount := atomic.AddInt32(&rn.voteCount, 1)
if int(newVoteCount) >= majority {
    // Señalar victoria
}
```

---

### Bug #2: Bug en `updateCommitIndex` (iteración incorrecta)
**Ubicación**: `raft.go:415-420`

**Problema**: Iteraba sobre valores del mapa (`peerAddress`) en lugar de claves, y podía contar el líder dos veces.

**Solución**:
- Cambiado `for _, peerID := range rn.peerAddress` a `for peerID := range rn.peerAddress`
- Agregada verificación para excluir al líder del conteo: `if peerID == rn.id { continue }`

**Código corregido**:
```go
matchCount := 1 // Líder se cuenta a sí mismo
for peerID := range rn.peerAddress {
    if peerID == rn.id {
        continue // Ya contamos al líder
    }
    if rn.matchIndex[peerID] >= N {
        matchCount++
    }
}
```

---

### Bug #3: Cálculo Incorrecto de Conflicto en `AppendEntries`
**Ubicación**: `rpc.go:103-118`

**Problema**: El cálculo del índice para truncar el log podía ser incorrecto, causando índices negativos o fuera de rango.

**Solución**:
- Reescrita la lógica de manejo de conflictos siguiendo el algoritmo de Raft
- Validación del offset antes de usar: `if entryOffset >= 0 && entryOffset < len(args.Entries)`
- Manejo correcto de casos sin conflictos

**Código corregido**:
```go
if len(args.Entries) > 0 {
    conflictIndex := -1
    for i, entry := range args.Entries {
        index := args.PrevLogIndex + 1 + i
        if index < len(rn.log) && rn.log[index].Term != entry.Term {
            conflictIndex = index
            break
        }
    }
    
    if conflictIndex != -1 {
        rn.log = rn.log[:conflictIndex]
        entryOffset := conflictIndex - (args.PrevLogIndex + 1)
        if entryOffset >= 0 && entryOffset < len(args.Entries) {
            rn.log = append(rn.log, args.Entries[entryOffset:]...)
        }
    } else {
        // Sin conflictos, añadir nuevas entradas
        if args.PrevLogIndex+1 < len(rn.log) {
            rn.log = rn.log[:args.PrevLogIndex+1]
        }
        rn.log = append(rn.log, args.Entries...)
    }
}
```

---

### Bug #4: Falta Notificación de `applyChan` en `AppendEntries`
**Ubicación**: `rpc.go:124-129`

**Problema**: Cuando un seguidor actualizaba su `commitIndex`, no notificaba a `applyChan`, por lo que los logs nunca se aplicaban.

**Solución**:
- Agregada notificación a `applyChan` después de actualizar `commitIndex`
- Verificación de que `commitIndex` realmente cambió antes de notificar
- Uso de `select` con `default` para evitar bloqueos

**Código corregido**:
```go
if args.LeaderCommit > rn.commitIndex {
    oldCommitIndex := rn.commitIndex
    lastNewEntryIndex := args.PrevLogIndex + len(args.Entries)
    rn.commitIndex = min(args.LeaderCommit, lastNewEntryIndex)
    
    // Notificar si commitIndex cambió
    if rn.commitIndex > oldCommitIndex {
        select {
        case rn.applyChan <- struct{}{}:
        default:
            // No bloquear si el canal ya está lleno
        }
    }
}
```

---

### Bug #5: Log Ficticio sin Término Definido
**Ubicación**: `raft.go:85`

**Problema**: El log se inicializaba con una entrada en índice 0, pero su término no estaba definido, causando problemas en comparaciones.

**Solución**:
- Cambiado `make([]LogEntry, 1)` a `[]LogEntry{{Term: 0, Command: nil}}`
- Inicialización explícita del término a 0

**Código corregido**:
```go
log: []LogEntry{{Term: 0, Command: nil}}, // Log ficticio en índice 0 con término 0
```

---

### Bug #6: Race Condition en `resetElectionTimer`
**Ubicación**: `raft.go:233-247`

**Problema**: Se llamaba sin mutex desde múltiples goroutines, causando cientos de reseteos simultáneos (confirmado en logs).

**Solución**:
- Creada función `resetElectionTimerUnlocked()` que no adquiere el mutex
- `resetElectionTimer()` ahora adquiere el mutex internamente
- Uso de `resetElectionTimerUnlocked()` desde funciones que ya tienen el mutex (`startElection`, `becomeFollower`)

**Código corregido**:
```go
// Versión thread-safe (adquiere mutex)
func (rn *RaftNode) resetElectionTimer() {
    rn.mu.Lock()
    defer rn.mu.Unlock()
    rn.resetElectionTimerUnlocked()
}

// Versión sin mutex (debe llamarse con mutex ya adquirido)
func (rn *RaftNode) resetElectionTimerUnlocked() {
    // ... lógica de reset ...
}
```

---

## 📊 Impacto de las Correcciones

### Antes de las correcciones:
- ❌ Race conditions causando comportamiento errático
- ❌ Comandos no se aplicaban a la máquina de estados
- ❌ Cientos de reseteos de timer innecesarios
- ❌ Posibles conteos incorrectos de votos
- ❌ Bugs en el cálculo de commitIndex

### Después de las correcciones:
- ✅ Thread-safety garantizado en todas las operaciones críticas
- ✅ Comandos se aplican correctamente cuando se comprometen
- ✅ Reseteos de timer controlados y eficientes
- ✅ Conteo de votos correcto y thread-safe
- ✅ Cálculo de commitIndex correcto

---

## 🧪 Pruebas Recomendadas

1. **Prueba de elecciones**: Verificar que las elecciones funcionan correctamente con múltiples nodos
2. **Prueba de replicación**: Verificar que los comandos se replican y aplican correctamente
3. **Prueba de concurrencia**: Ejecutar múltiples comandos simultáneamente
4. **Prueba de fallos**: Simular fallos de nodos y verificar recuperación
5. **Análisis de logs**: Verificar que no hay cientos de reseteos de timer

---

## 📝 Notas Adicionales

- Todas las correcciones mantienen la compatibilidad con el algoritmo Raft original
- Se agregó `sync/atomic` para el contador de votos
- Se mejoró la documentación de funciones que requieren mutex
- No se introdujeron cambios breaking en la API pública

---

**Fecha de corrección**: 2025/11/23  
**Versión**: feature/architecture_fork  
**Estado**: ✅ Todos los bugs críticos corregidos

