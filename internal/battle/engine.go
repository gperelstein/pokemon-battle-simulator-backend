package battle

import (
	"errors"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/gen"
)

// Engine ejecuta la lógica de la batalla. Es deliberadamente sincrónico y
// stateless: toda la información vive en *State, y cada llamada a Apply es
// una transición determinística (state, input) → (state', events).
//
// Concurrencia: el Engine no toma locks. El llamador (paquete session)
// serializa el acceso a una batalla concreta.
type Engine struct {
	Dex   dex.Dex
	Rules gen.Rules
	// Registro de efectos (moves/abilities/items custom). Si un id no está
	// registrado se aplica comportamiento por defecto y se loggea.
	// Effects effect.Registry
}

// New construye un Engine para una generación concreta.
func New(d dex.Dex, rules gen.Rules) *Engine {
	return &Engine{Dex: d, Rules: rules}
}

// Start inicializa el State a partir de los dos equipos y el seed. Deja la
// batalla en PhaseAwaitingActions con el primer Pokémon de cada lado activo.
// Emite los eventos iniciales (switch-in, entry hazards no aplican al inicio,
// abilities on-entry como Intimidate sí).
func (e *Engine) Start(state *State) ([]Event, error) {
	panic("not implemented")
}

// Apply procesa una Action del jugador y avanza el estado tanto como pueda
// sin nuevo input. Devuelve los eventos generados y el próximo input esperado
// implícito en State.Phase / State.PendingSwitches.
//
// Errores:
//   - acción inválida para la fase actual
//   - move sin PP, target inválido, switch a Pokémon debilitado o atrapado
//   - lado equivocado (no es tu turno en forced switch)
//
// Flujo típico:
//
//	PhaseAwaitingActions:
//	  guarda la acción en PendingActions[side]
//	  si ambos lados ya enviaron → encola switches+moves en orden y entra a
//	    PhaseResolving, luego procesa hasta agotar la cola o pausar
//
//	PhaseAwaitingForcedSwitch:
//	  valida que side esté en PendingSwitches
//	  ejecuta el switch, lo quita de PendingSwitches
//	  si quedan switches pendientes, sigue esperando
//	  si no, vuelve a PhaseResolving y continúa la cola
func (e *Engine) Apply(state *State, action Action) ([]Event, error) {
	panic("not implemented")
}

// process drena la cola en PhaseResolving hasta que (a) se vacía y pasa a
// PhaseEndOfTurn → PhaseAwaitingActions del próximo turno, o (b) encuentra
// un QueueRequestForcedSwitch y deja al State pausado en
// PhaseAwaitingForcedSwitch.
func (e *Engine) process(state *State) []Event {
	panic("not implemented")
}

// Errores típicos exportados para que el transporte pueda mapearlos a
// códigos de error del protocolo.
var (
	ErrWrongPhase        = errors.New("battle: action not allowed in current phase")
	ErrWrongSide         = errors.New("battle: not your turn")
	ErrInvalidMoveSlot   = errors.New("battle: invalid move slot")
	ErrNoPP              = errors.New("battle: move has no PP left")
	ErrInvalidSwitchSlot = errors.New("battle: invalid switch target")
	ErrTrapped           = errors.New("battle: active pokemon is trapped")
)
