package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/battle"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/protocol"
)

// sendBuffer es el tamaño del buffer de salida por conexión. Si se llena
// (cliente lento), la conexión se cierra para no bloquear al resto.
const sendBuffer = 64

// Conn es el estado por conexión WebSocket. Tiene su propia goroutine de
// lectura (Server.readLoop) y una de escritura (writeLoop) que drena `send`,
// de modo que las escrituras quedan serializadas aunque otra goroutine (la del
// rival) emita eventos a esta conexión.
//
// El cierre se coordina con el canal `done` (cerrado una sola vez vía
// closeOnce); nunca se cierra `send`, así un emisor concurrente no puede
// panickear por "send on closed channel".
type Conn struct {
	server *Server
	ws     *websocket.Conn
	send   chan []byte
	done   chan struct{}

	playerID string
	name     string

	mu        sync.Mutex // protege name, format y los campos de batalla
	format    string
	battleID  string
	side      battle.SideID
	closeOnce sync.Once
}

func newConn(s *Server, c *websocket.Conn, playerID string) *Conn {
	return &Conn{
		server:   s,
		ws:       c,
		send:     make(chan []byte, sendBuffer),
		done:     make(chan struct{}),
		playerID: playerID,
	}
}

// sendMsg serializa y encola un mensaje hacia el cliente. Si el buffer está
// lleno (cliente demasiado lento), cierra la conexión.
func (c *Conn) sendMsg(msg protocol.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	case <-c.done:
	default:
		c.close()
	}
}

func (c *Conn) sendError(code, message string) {
	c.sendMsg(protocol.ServerMessage{Type: "error", Error: &protocol.ErrorMsg{Code: code, Message: message}})
}

func (c *Conn) setName(name string) {
	c.mu.Lock()
	c.name = name
	c.mu.Unlock()
}

func (c *Conn) displayName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.name != "" {
		return c.name
	}
	return c.playerID
}

func (c *Conn) setFormat(format string) {
	c.mu.Lock()
	c.format = format
	c.mu.Unlock()
}

func (c *Conn) currentFormat() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.format
}

func (c *Conn) setBattle(id string, side battle.SideID) {
	c.mu.Lock()
	c.battleID = id
	c.side = side
	c.mu.Unlock()
}

func (c *Conn) battleInfo() (string, battle.SideID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.battleID, c.side
}

func (c *Conn) clearBattle() {
	c.mu.Lock()
	c.battleID = ""
	c.mu.Unlock()
}

// writeLoop drena `send` escribiendo al socket hasta que se cierra `done`.
func (c *Conn) writeLoop() {
	for {
		select {
		case data := <-c.send:
			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				c.close()
				return
			}
		case <-c.done:
			c.ws.Close()
			return
		}
	}
}

// close cierra la conexión una sola vez (idempotente).
func (c *Conn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.ws.Close()
	})
}
