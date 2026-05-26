// Package ws implementa el transporte WebSocket: upgrade HTTP, lifecycle de
// conexión, lectura/escritura de mensajes del protocolo y dispatch al
// matchmaking / session correspondiente.
//
// Esqueleto a completar. Dependencia recomendada: gorilla/websocket.
package ws

import (
	"net/http"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/matchmaking"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/session"
)

// Server agrupa las dependencias del transporte. Una sola instancia para
// todo el proceso.
type Server struct {
	Store *session.Store
	Queue *matchmaking.Queue
	// engineFactory func(gen int) *battle.Engine  // crea engines on-demand al iniciar batallas
}

// Handler devuelve el http.Handler que hace upgrade a WebSocket y maneja la
// conexión. Cada conexión vive en su propia goroutine.
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. upgrade a WebSocket
		// 2. crear *Conn (estado por conexión: playerID, batalla actual)
		// 3. loop de lectura: parsear ClientMessage, dispatch
		// 4. al cerrar: limpiar de queue y notificar forfeit si está en batalla
		panic("not implemented")
	}
}
