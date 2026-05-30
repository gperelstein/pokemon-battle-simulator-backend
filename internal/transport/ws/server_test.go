package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/battle"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/protocol"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/transport/ws"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	d, err := dex.Load("../../dex/testdata")
	if err != nil {
		t.Fatalf("dex.Load: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.New(d).Handler())
	return httptest.NewServer(mux)
}

func dial(t *testing.T, srv *httptest.Server) *gws.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, _, err := gws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func send(t *testing.T, c *gws.Conn, msg protocol.ClientMessage) {
	t.Helper()
	data, _ := json.Marshal(msg)
	if err := c.WriteMessage(gws.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readUntil lee mensajes del servidor hasta encontrar uno del tipo pedido (o
// falla por timeout).
func readUntil(t *testing.T, c *gws.Conn, typ string) protocol.ServerMessage {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		c.SetReadDeadline(deadline)
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("esperando %q: %v", typ, err)
		}
		var m protocol.ServerMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m.Type == typ {
			return m
		}
	}
}

func teamJSON(species, move string) json.RawMessage {
	var tm pokemon.Team
	tm.Members[0] = pokemon.Pokemon{SpeciesID: species, Level: 50, Moves: [4]string{move}}
	raw, _ := json.Marshal(tm)
	return raw
}

func joinQueue(t *testing.T, c *gws.Conn, name, species, move string) {
	t.Helper()
	send(t, c, protocol.ClientMessage{Type: "auth", Auth: &protocol.AuthMsg{Name: name}})
	send(t, c, protocol.ClientMessage{Type: "queueJoin", QueueJoin: &protocol.QueueJoinMsg{
		Format: "gen9", Team: teamJSON(species, move),
	}})
}

func sendMove(t *testing.T, c *gws.Conn, battleID string, slot int) {
	t.Helper()
	action, _ := json.Marshal(battle.Action{Kind: battle.ActionMove, Move: &battle.MoveAction{MoveSlot: slot}})
	send(t, c, protocol.ClientMessage{Type: "battleAction", BattleAction: &protocol.BattleActionMsg{
		BattleID: battleID, Action: action,
	}})
}

func TestMatchmakingAndTurn(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := dial(t, srv)
	defer a.Close()
	b := dial(t, srv)
	defer b.Close()

	joinQueue(t, a, "Ash", "charizard", "tackle")
	joinQueue(t, b, "Gary", "pikachu", "tackle")

	matchedA := readUntil(t, a, "queueMatched")
	matchedB := readUntil(t, b, "queueMatched")
	if matchedA.QueueMatched.BattleID != matchedB.QueueMatched.BattleID {
		t.Fatalf("battleId distinto: %s vs %s", matchedA.QueueMatched.BattleID, matchedB.QueueMatched.BattleID)
	}
	if matchedA.QueueMatched.YourSide == matchedB.QueueMatched.YourSide {
		t.Fatalf("ambos en el mismo lado: %d", matchedA.QueueMatched.YourSide)
	}
	battleID := matchedA.QueueMatched.BattleID

	// Ambos reciben el request de acción del turno 1.
	reqA := readUntil(t, a, "battleRequest")
	reqB := readUntil(t, b, "battleRequest")
	if reqA.BattleRequest.Kind != "action" || reqB.BattleRequest.Kind != "action" {
		t.Fatalf("requests turno 1: A=%q B=%q, want action", reqA.BattleRequest.Kind, reqB.BattleRequest.Kind)
	}

	// Ambos atacan (tackle no noquea) → el turno se resuelve y llega el request 2.
	sendMove(t, a, battleID, 0)
	sendMove(t, b, battleID, 0)

	stateA := readUntil(t, a, "battleState")
	reqA2 := readUntil(t, a, "battleRequest")
	if reqA2.BattleRequest.Kind != "action" {
		t.Fatalf("request turno 2 = %q, want action", reqA2.BattleRequest.Kind)
	}

	// La vista debe reflejar el turno 2.
	var view struct {
		Turn int `json:"turn"`
	}
	raw, _ := json.Marshal(stateA.BattleState.View)
	_ = json.Unmarshal(raw, &view)
	if view.Turn != 2 {
		t.Errorf("turno en la vista = %d, want 2", view.Turn)
	}
}

func TestForfeitOnDisconnect(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := dial(t, srv)
	b := dial(t, srv)
	defer b.Close()

	joinQueue(t, a, "Ash", "charizard", "tackle")
	joinQueue(t, b, "Gary", "pikachu", "tackle")

	readUntil(t, a, "queueMatched")
	readUntil(t, b, "queueMatched")
	readUntil(t, a, "battleRequest")
	readUntil(t, b, "battleRequest")

	// A se desconecta abruptamente → B debe enterarse del fin de la batalla.
	a.Close()

	evs := readUntil(t, b, "battleEvents")
	if !hasEventKind(evs.BattleEvents.Events, battle.EventBattleEnded) {
		t.Error("B debería recibir el evento BattleEnded tras la desconexión de A")
	}
}

// hasEventKind busca un evento (map JSON) con el Kind dado en una lista de
// eventos serializados.
func hasEventKind(events []any, kind battle.EventKind) bool {
	for _, e := range events {
		if m, ok := e.(map[string]any); ok {
			if n, ok := m["Kind"].(float64); ok && int(n) == int(kind) {
				return true
			}
		}
	}
	return false
}
