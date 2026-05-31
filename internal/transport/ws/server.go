// Package ws implementa el transporte WebSocket: upgrade HTTP, lifecycle de
// conexión, lectura/escritura de mensajes del protocolo y dispatch al
// matchmaking / session correspondiente.
//
// Modelo de concurrencia: una goroutine de lectura y otra de escritura por
// conexión. El motor (battle.Engine) es sincrónico; el acceso a cada batalla se
// serializa con session.Battle.WithLock. Las vistas/eventos a enviar se arman
// DENTRO del lock (snapshot consistente) y se encolan FUERA (solo push a canal).
package ws

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/battle"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/matchmaking"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/protocol"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/session"
)

// Server agrupa las dependencias del transporte. Una sola instancia para todo
// el proceso.
type Server struct {
	Store *session.Store
	Queue *matchmaking.Queue
	Dex   dex.Dex
}

// New construye el servidor con sus dependencias.
func New(d dex.Dex) *Server {
	return &Server{Store: session.NewStore(), Queue: matchmaking.New(), Dex: d}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// MVP: aceptar cualquier origen. Endurecer cuando exista el frontend.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler devuelve el http.Handler que hace upgrade a WebSocket. Cada conexión
// vive en sus propias goroutines (lectura + escritura).
func (s *Server) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // Upgrade ya respondió el error HTTP
		}
		conn := newConn(s, c, randomID())
		go conn.writeLoop()
		s.readLoop(conn)
	}
}

// readLoop lee mensajes del cliente y los despacha hasta que la conexión se
// cierra; entonces limpia (cola + forfeit si estaba en batalla).
func (s *Server) readLoop(c *Conn) {
	defer s.handleDisconnect(c)
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var msg protocol.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.sendError("bad_json", "no se pudo parsear el mensaje")
			continue
		}
		s.dispatch(c, &msg)
	}
}

func (s *Server) dispatch(c *Conn, msg *protocol.ClientMessage) {
	switch msg.Type {
	case "auth":
		if msg.Auth != nil {
			c.setName(msg.Auth.Name)
		}
	case "queueJoin":
		s.handleQueueJoin(c, msg.QueueJoin)
	case "queueLeave":
		s.handleQueueLeave(c)
	case "battleAction":
		s.handleBattleAction(c, msg.BattleAction)
	case "battleForfeit":
		s.handleForfeit(c)
	default:
		c.sendError("unknown_type", "tipo de mensaje desconocido: "+msg.Type)
	}
}

func (s *Server) handleQueueJoin(c *Conn, msg *protocol.QueueJoinMsg) {
	if msg == nil {
		c.sendError("bad_request", "falta queueJoin")
		return
	}
	if id, _ := c.battleInfo(); id != "" {
		c.sendError("already_in_battle", "ya estás en una batalla")
		return
	}
	genID := parseGen(msg.Format)

	var team pokemon.Team
	if err := json.Unmarshal(msg.Team, &team); err != nil {
		c.sendError("bad_team", "no se pudo parsear el equipo")
		return
	}
	if err := pokemon.ValidateTeam(team, gen.For(genID), s.Dex); err != nil {
		c.sendError("invalid_team", err.Error())
		return
	}

	c.setFormat(msg.Format)
	w := &matchmaking.Waiter{PlayerID: c.playerID, TeamRaw: msg.Team, Conn: c}
	if other := s.Queue.Enqueue(msg.Format, w); other != nil {
		s.startBattle(msg.Format, genID, other, w)
	}
	// Si no hubo match, el jugador queda esperando (sin mensaje extra).
}

func (s *Server) handleQueueLeave(c *Conn) {
	if f := c.currentFormat(); f != "" {
		s.Queue.Remove(f, c.playerID)
		c.setFormat("")
	}
}

// startBattle crea la batalla a partir de dos waiters ya emparejados (a = el que
// esperaba, lado A; b = el que acaba de entrar, lado B), la inicia y notifica a
// ambos.
func (s *Server) startBattle(format string, genID int, a, b *matchmaking.Waiter) {
	connA, _ := a.Conn.(*Conn)
	connB, _ := b.Conn.(*Conn)
	if connA == nil || connB == nil {
		return
	}

	var teamA, teamB pokemon.Team
	if json.Unmarshal(a.TeamRaw, &teamA) != nil || json.Unmarshal(b.TeamRaw, &teamB) != nil {
		connA.sendError("internal", "equipo inválido al crear la batalla")
		connB.sendError("internal", "equipo inválido al crear la batalla")
		return
	}

	id := randomID()
	seed := randomSeed()
	state := battle.NewState(genID, seed, format,
		battle.TeamSetup{Player: battle.PlayerInfo{ID: a.PlayerID, Name: connA.displayName()}, Team: teamA},
		battle.TeamSetup{Player: battle.PlayerInfo{ID: b.PlayerID, Name: connB.displayName()}, Team: teamB},
	)
	bt := &session.Battle{ID: id, State: state, Engine: battle.New(s.Dex, gen.For(genID)), ConnA: connA, ConnB: connB}

	var initEvents []battle.Event
	var startErr error
	var out [2][]protocol.ServerMessage
	bt.WithLock(func(st *battle.State, e *battle.Engine) {
		initEvents, startErr = e.Start(st)
		if startErr == nil {
			out = buildBroadcast(id, st, initEvents)
		}
	})
	if startErr != nil {
		connA.sendError("internal", startErr.Error())
		connB.sendError("internal", startErr.Error())
		return
	}

	s.Store.Add(bt)
	connA.setBattle(id, battle.SideA)
	connB.setBattle(id, battle.SideB)
	connA.setFormat("")
	connB.setFormat("")

	connA.sendMsg(protocol.ServerMessage{Type: "queueMatched", QueueMatched: &protocol.QueueMatchedMsg{
		BattleID: id, OpponentID: b.PlayerID, YourSide: int(battle.SideA), Seed: seed,
	}})
	connB.sendMsg(protocol.ServerMessage{Type: "queueMatched", QueueMatched: &protocol.QueueMatchedMsg{
		BattleID: id, OpponentID: a.PlayerID, YourSide: int(battle.SideB), Seed: seed,
	}})
	sendBroadcast(bt, out)
}

func (s *Server) handleBattleAction(c *Conn, msg *protocol.BattleActionMsg) {
	if msg == nil {
		c.sendError("bad_request", "falta battleAction")
		return
	}
	id, side := c.battleInfo()
	if id == "" {
		c.sendError("no_battle", "no estás en una batalla")
		return
	}
	bt, ok := s.Store.Get(id)
	if !ok {
		c.sendError("no_battle", "la batalla no existe")
		return
	}

	var action battle.Action
	if err := json.Unmarshal(msg.Action, &action); err != nil {
		c.sendError("bad_action", "no se pudo parsear la acción")
		return
	}
	action.Side = side // seguridad: el lado lo fija el servidor, no el cliente

	s.applyAction(bt, action, c)
}

func (s *Server) handleForfeit(c *Conn) {
	id, side := c.battleInfo()
	if id == "" {
		c.sendError("no_battle", "no estás en una batalla")
		return
	}
	if bt, ok := s.Store.Get(id); ok {
		s.applyAction(bt, battle.Action{Kind: battle.ActionForfeit, Side: side}, c)
	}
}

// applyAction corre Apply bajo el lock de la batalla, arma los mensajes para
// ambos lados dentro del lock y los envía fuera. Si la batalla terminó, la
// limpia.
func (s *Server) applyAction(bt *session.Battle, action battle.Action, from *Conn) {
	var applyErr error
	var out [2][]protocol.ServerMessage
	var ended, waiting bool
	bt.WithLock(func(st *battle.State, e *battle.Engine) {
		if st.Phase == battle.PhaseEnded {
			ended = true
			return
		}
		var events []battle.Event
		events, applyErr = e.Apply(st, action)
		if applyErr != nil {
			return
		}
		// Acción guardada esperando al rival (sin nada que resolver todavía):
		// no se difunde nada; solo se le avisa "wait" al que eligió.
		if len(events) == 0 && st.Phase == battle.PhaseAwaitingActions {
			waiting = true
			return
		}
		out = buildBroadcast(bt.ID, st, events)
		ended = st.Phase == battle.PhaseEnded
	})
	if applyErr != nil {
		from.sendError("invalid_action", applyErr.Error())
		return
	}
	if waiting {
		from.sendMsg(protocol.ServerMessage{Type: "battleRequest", BattleRequest: &protocol.BattleRequestMsg{
			BattleID: bt.ID, Kind: "wait",
		}})
		return
	}
	sendBroadcast(bt, out)
	if ended {
		s.cleanupBattle(bt)
	}
}

// handleDisconnect limpia al cerrarse una conexión: la saca de la cola y, si
// estaba en batalla, la rinde (gana el rival).
func (s *Server) handleDisconnect(c *Conn) {
	if f := c.currentFormat(); f != "" {
		s.Queue.Remove(f, c.playerID)
	}
	if id, side := c.battleInfo(); id != "" {
		if bt, ok := s.Store.Get(id); ok {
			s.applyAction(bt, battle.Action{Kind: battle.ActionForfeit, Side: side}, c)
		}
	}
	c.close()
}

func (s *Server) cleanupBattle(bt *session.Battle) {
	s.Store.Remove(bt.ID)
	if c, ok := bt.ConnA.(*Conn); ok && c != nil {
		c.clearBattle()
	}
	if c, ok := bt.ConnB.(*Conn); ok && c != nil {
		c.clearBattle()
	}
}

// --- broadcast ---

// buildBroadcast arma, para cada lado, los mensajes a enviar tras un Apply:
// snapshot (vista propia), eventos y, si corresponde, el request. Debe llamarse
// con el lock de la batalla tomado (lee el state).
func buildBroadcast(battleID string, state *battle.State, events []battle.Event) [2][]protocol.ServerMessage {
	rawEvents := make([]any, len(events))
	for i := range events {
		rawEvents[i] = events[i]
	}

	var out [2][]protocol.ServerMessage
	for s := range out {
		side := battle.SideID(s)
		msgs := []protocol.ServerMessage{
			{Type: "battleState", BattleState: &protocol.BattleStateMsg{BattleID: battleID, View: buildView(state, side)}},
			{Type: "battleEvents", BattleEvents: &protocol.BattleEventsMsg{BattleID: battleID, Events: rawEvents}},
		}
		if req := buildRequest(battleID, state, side); req != nil {
			msgs = append(msgs, protocol.ServerMessage{Type: "battleRequest", BattleRequest: req})
		}
		out[s] = msgs
	}
	return out
}

// sendBroadcast envía a cada conexión su lista de mensajes (fuera del lock).
func sendBroadcast(bt *session.Battle, out [2][]protocol.ServerMessage) {
	if c, ok := bt.ConnA.(*Conn); ok && c != nil {
		for _, m := range out[battle.SideA] {
			c.sendMsg(m)
		}
	}
	if c, ok := bt.ConnB.(*Conn); ok && c != nil {
		for _, m := range out[battle.SideB] {
			c.sendMsg(m)
		}
	}
}

// buildRequest indica qué se espera de side ahora, o nil si nada (batalla
// terminada).
func buildRequest(battleID string, state *battle.State, side battle.SideID) *protocol.BattleRequestMsg {
	switch state.Phase {
	case battle.PhaseAwaitingActions:
		return &protocol.BattleRequestMsg{
			BattleID:    battleID,
			Kind:        "action",
			ValidMoves:  validMoves(&state.Sides[side]),
			ValidSwitch: validSwitches(&state.Sides[side]),
		}
	case battle.PhaseAwaitingForcedSwitch:
		if containsSide(state.PendingSwitches, side) {
			return &protocol.BattleRequestMsg{
				BattleID:    battleID,
				Kind:        "forcedSwitch",
				ValidSwitch: validSwitches(&state.Sides[side]),
			}
		}
		return &protocol.BattleRequestMsg{BattleID: battleID, Kind: "wait"}
	default:
		return nil
	}
}

func validMoves(side *battle.Side) []int {
	active := &side.Team[side.Active]
	var slots []int
	for i, id := range active.Set.Moves {
		if id != "" && active.PP[i] > 0 {
			slots = append(slots, i)
		}
	}
	return slots
}

func validSwitches(side *battle.Side) []int {
	var slots []int
	for i := range side.Team {
		bp := &side.Team[i]
		if !bp.Empty && !bp.Fainted && i != side.Active {
			slots = append(slots, i)
		}
	}
	return slots
}

func containsSide(sides []battle.SideID, side battle.SideID) bool {
	for _, s := range sides {
		if s == side {
			return true
		}
	}
	return false
}

// --- helpers ---

// parseGen extrae el número de generación de un formato tipo "gen9ou".
// Default 9 si no se reconoce.
func parseGen(format string) int {
	f := strings.ToLower(format)
	if !strings.HasPrefix(f, "gen") {
		return gen.MaxGen
	}
	digits := ""
	for _, r := range f[3:] {
		if r < '0' || r > '9' {
			break
		}
		digits += string(r)
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < gen.MinGen || n > gen.MaxGen {
		return gen.MaxGen
	}
	return n
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func randomSeed() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint64(b[:])
}
