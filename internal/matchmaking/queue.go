// Package matchmaking implementa una cola de espera simple por formato.
// MVP: FIFO. Cuando hay dos jugadores con el mismo formato, se crea una
// batalla en session.Store y se notifica a ambos.
package matchmaking

import (
	"encoding/json"
	"sync"
)

// Queue es una cola FIFO por formato. Cada Enqueue puede devolver un match
// inmediatamente si ya había alguien esperando ese formato.
type Queue struct {
	mu      sync.Mutex
	waiting map[string]*Waiter // format → waiter esperando
}

type Waiter struct {
	PlayerID string
	TeamRaw  json.RawMessage // pokemon.Team serializado, sin parsear todavía
	// Conn es la conexión del jugador en espera (*ws.Conn, guardado como any
	// para evitar el ciclo de imports ws→matchmaking). Quien arma el match la
	// usa para notificar a este jugador.
	Conn any
}

func New() *Queue {
	return &Queue{waiting: map[string]*Waiter{}}
}

// Enqueue agrega un waiter. Si ya había otro para el mismo formato, devuelve
// el oponente y lo saca de la cola; el llamador crea la batalla y notifica
// a ambos. Si no, agrega al waiter actual y devuelve nil.
func (q *Queue) Enqueue(format string, w *Waiter) *Waiter {
	q.mu.Lock()
	defer q.mu.Unlock()
	if other, ok := q.waiting[format]; ok {
		delete(q.waiting, format)
		return other
	}
	q.waiting[format] = w
	return nil
}

// Remove saca al jugador de la cola (por desconexión o queueLeave).
func (q *Queue) Remove(format, playerID string) {
	q.mu.Lock()
	if w, ok := q.waiting[format]; ok && w.PlayerID == playerID {
		delete(q.waiting, format)
	}
	q.mu.Unlock()
}
