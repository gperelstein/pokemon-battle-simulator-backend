// Package session mantiene el registro en memoria de batallas activas y
// conexiones de jugadores. Es el dueño de los locks por batalla: el motor
// (battle.Engine) es sincrónico y no toma locks; este paquete serializa el
// acceso para que cada batalla procese una acción a la vez.
package session

import (
	"sync"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/battle"
)

// Store guarda las batallas activas indexadas por id. Los métodos son seguros
// para uso concurrente.
type Store struct {
	mu       sync.RWMutex
	battles  map[string]*Battle
}

// Battle es el wrapper que vive en memoria: el state, el engine y un lock
// propio para serializar las llamadas a Apply.
type Battle struct {
	ID     string
	State  *battle.State
	Engine *battle.Engine

	// connections: dos conexiones (una por jugador) para emitir eventos.
	// Tipo concreto definido en transport/ws para evitar dependencia inversa.
	ConnA, ConnB any

	mu sync.Mutex
}

func NewStore() *Store {
	return &Store{battles: map[string]*Battle{}}
}

func (s *Store) Add(b *Battle) {
	s.mu.Lock()
	s.battles[b.ID] = b
	s.mu.Unlock()
}

func (s *Store) Get(id string) (*Battle, bool) {
	s.mu.RLock()
	b, ok := s.battles[id]
	s.mu.RUnlock()
	return b, ok
}

func (s *Store) Remove(id string) {
	s.mu.Lock()
	delete(s.battles, id)
	s.mu.Unlock()
}

// WithLock ejecuta fn con el lock de la batalla tomado. Patrón: todo Apply
// del Engine ocurre dentro de WithLock.
func (b *Battle) WithLock(fn func(*battle.State, *battle.Engine)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fn(b.State, b.Engine)
}
