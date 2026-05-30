// Package protocol define los mensajes JSON intercambiados entre cliente y
// servidor por WebSocket. Es la única superficie pública del backend para el
// frontend; mantenerla estable y bien tipada es importante.
//
// Forma general: cada mensaje tiene un campo "type" que discrimina el resto.
// La (de)serialización concreta vivirá en transport/ws; acá solo definimos
// las estructuras.
package protocol

import "encoding/json"

// ───── Cliente → Servidor ─────

type ClientMessage struct {
	Type string `json:"type"`

	Auth          *AuthMsg          `json:"auth,omitempty"`
	QueueJoin     *QueueJoinMsg     `json:"queueJoin,omitempty"`
	QueueLeave    *struct{}         `json:"queueLeave,omitempty"`
	BattleAction  *BattleActionMsg  `json:"battleAction,omitempty"`
	BattleForfeit *BattleForfeitMsg `json:"battleForfeit,omitempty"`
}

type AuthMsg struct {
	Token string `json:"token"` // MVP: token opaco
	Name  string `json:"name"`
}

type QueueJoinMsg struct {
	Format string          `json:"format"` // "gen9ou", "gen3uu", etc.
	Team   json.RawMessage `json:"team"`   // pokemon.Team serializado; crudo para no acoplar el protocolo a pokemon
}

type BattleActionMsg struct {
	BattleID string          `json:"battleId"`
	Action   json.RawMessage `json:"action"` // battle.Action serializado; el transporte lo deserializa
}

type BattleForfeitMsg struct {
	BattleID string `json:"battleId"`
}

// ───── Servidor → Cliente ─────

type ServerMessage struct {
	Type string `json:"type"`

	QueueMatched  *QueueMatchedMsg  `json:"queueMatched,omitempty"`
	BattleState   *BattleStateMsg   `json:"battleState,omitempty"`
	BattleEvents  *BattleEventsMsg  `json:"battleEvents,omitempty"`
	BattleRequest *BattleRequestMsg `json:"battleRequest,omitempty"`
	Error         *ErrorMsg         `json:"error,omitempty"`
}

type QueueMatchedMsg struct {
	BattleID   string `json:"battleId"`
	OpponentID string `json:"opponentId"`
	YourSide   int    `json:"yourSide"` // 0 o 1
	Seed       uint64 `json:"seed"`
}

// BattleStateMsg es un snapshot público para un jugador concreto. Oculta
// info privada del rival (HP exacto, set, item, ability) según las reglas
// del formato.
type BattleStateMsg struct {
	BattleID string `json:"battleId"`
	View     any    `json:"view"` // tipo definido cuando exista el "public view"
}

type BattleEventsMsg struct {
	BattleID string `json:"battleId"`
	Events   []any  `json:"events"` // battle.Event serializados
}

// BattleRequestMsg le dice al cliente qué se espera de él ahora.
//   - "action": elegir Move o Switch normal
//   - "forcedSwitch": elegir Switch obligatorio
//   - "wait": el otro jugador todavía no eligió
type BattleRequestMsg struct {
	BattleID    string `json:"battleId"`
	Kind        string `json:"kind"`
	ValidMoves  []int  `json:"validMoves,omitempty"`  // slots utilizables
	ValidSwitch []int  `json:"validSwitch,omitempty"` // slots de equipo elegibles
	Trapped     bool   `json:"trapped,omitempty"`
}

type ErrorMsg struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
