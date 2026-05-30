package battle

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/pkg/rng"
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

// Start inicializa el State a partir de los dos equipos ya cableados (ver
// NewState): calcula stats finales y HP, inicializa PP, y deja la batalla en
// PhaseAwaitingActions (Turn 1) con el primer Pokémon de cada lado activo.
// Emite los switch-in iniciales y el inicio de turno.
//
// (Las abilities on-entry tipo Intimidate llegan en un paso posterior.)
func (e *Engine) Start(state *State) ([]Event, error) {
	for side := range state.Sides {
		s := &state.Sides[side]
		for i := range s.Team {
			bp := &s.Team[i]
			if bp.Empty || bp.Set.SpeciesID == "" {
				bp.Empty = true
				continue
			}
			sp, ok := e.Dex.Species(e.Rules.Gen, bp.Set.SpeciesID)
			if !ok {
				return nil, fmt.Errorf("battle: species %q no existe en gen %d", bp.Set.SpeciesID, e.Rules.Gen)
			}
			maxHP, stats := computeStats(sp.BaseStats, bp.Set, e.Rules)
			bp.MaxHP, bp.HP, bp.Stats = maxHP, maxHP, stats
			if bp.Volatiles == nil {
				bp.Volatiles = map[string]any{}
			}
			for slot, moveID := range bp.Set.Moves {
				if moveID == "" {
					bp.PP[slot] = 0
					continue
				}
				if mv, ok := e.Dex.Move(e.Rules.Gen, moveID); ok {
					bp.PP[slot] = mv.PP
				}
			}
		}
	}

	state.Turn = 1
	state.Phase = PhaseAwaitingActions

	var evs []Event
	for side := range state.Sides {
		evs = append(evs, Event{Kind: EventSwitchIn, Side: SideID(side), Slot: state.Sides[side].Active})
	}
	evs = append(evs, Event{Kind: EventTurnStarted, Amount: state.Turn})
	return evs, nil
}

// Apply procesa una Action del jugador y avanza el estado tanto como pueda sin
// nuevo input. Devuelve los eventos generados; el próximo input esperado queda
// implícito en State.Phase / State.PendingSwitches.
func (e *Engine) Apply(state *State, action Action) ([]Event, error) {
	if state.Phase == PhaseEnded {
		return nil, ErrWrongPhase
	}
	if action.Kind == ActionForfeit {
		return e.forfeit(state, action.Side), nil
	}

	switch state.Phase {
	case PhaseAwaitingActions:
		return e.applyChoice(state, action)
	case PhaseAwaitingForcedSwitch:
		return e.applyForcedSwitch(state, action)
	default:
		return nil, ErrWrongPhase
	}
}

// applyChoice maneja una Action en PhaseAwaitingActions: valida, la guarda y, si
// ambos lados ya eligieron, resuelve el turno.
func (e *Engine) applyChoice(state *State, action Action) ([]Event, error) {
	if action.Kind != ActionMove && action.Kind != ActionSwitch {
		return nil, ErrWrongPhase
	}
	side := action.Side
	if err := e.validateChoice(state, action); err != nil {
		return nil, err
	}

	a := action
	state.PendingActions[side] = &a

	if state.PendingActions[SideA] == nil || state.PendingActions[SideB] == nil {
		return nil, nil // falta el otro lado
	}
	return e.resolveTurn(state), nil
}

func (e *Engine) validateChoice(state *State, action Action) error {
	side := action.Side
	active := e.active(state, side)
	switch action.Kind {
	case ActionMove:
		if action.Move == nil || action.Move.MoveSlot < 0 || action.Move.MoveSlot >= len(active.Set.Moves) {
			return ErrInvalidMoveSlot
		}
		if active.Set.Moves[action.Move.MoveSlot] == "" {
			return ErrInvalidMoveSlot
		}
		if active.PP[action.Move.MoveSlot] <= 0 {
			return ErrNoPP
		}
	case ActionSwitch:
		if action.Switch == nil {
			return ErrInvalidSwitchSlot
		}
		return e.validateSwitchTarget(state, side, action.Switch.TeamSlot)
	}
	return nil
}

// validateSwitchTarget verifica que el slot sea un Pokémon vivo, no vacío y
// distinto del activo. (El atrape — Mean Look, Arena Trap — llega más adelante.)
func (e *Engine) validateSwitchTarget(state *State, side SideID, slot int) error {
	s := &state.Sides[side]
	if slot < 0 || slot >= len(s.Team) {
		return ErrInvalidSwitchSlot
	}
	bp := &s.Team[slot]
	if bp.Empty || bp.Fainted || slot == s.Active {
		return ErrInvalidSwitchSlot
	}
	return nil
}

// resolveTurn construye la cola (switches voluntarios → moves por
// prioridad/velocidad) y la procesa.
func (e *Engine) resolveTurn(state *State) []Event {
	r := rng.New(turnSeed(state))

	var movers []SideID
	for _, side := range []SideID{SideA, SideB} {
		act := state.PendingActions[side]
		switch act.Kind {
		case ActionSwitch:
			state.Queue.PushBack(QueueItem{Kind: QueueSwitch, Side: side, Switch: act.Switch})
		case ActionMove:
			movers = append(movers, side)
		}
	}
	for _, side := range e.orderMovers(state, movers, r) {
		state.Queue.PushBack(QueueItem{
			Kind:     QueueMove,
			Side:     side,
			MoveSlot: state.PendingActions[side].Move.MoveSlot,
		})
	}

	state.PendingActions = [2]*Action{}
	state.Phase = PhaseResolving
	return e.process(state, r)
}

// orderMovers ordena los lados que usan move por prioridad (desc), luego
// velocidad (desc), y desempata con el RNG (coin flip de velocidad).
func (e *Engine) orderMovers(state *State, movers []SideID, r rng.RNG) []SideID {
	if len(movers) < 2 {
		return movers
	}
	prio := func(side SideID) int {
		act := state.PendingActions[side]
		mv, _ := e.Dex.Move(e.Rules.Gen, e.active(state, side).Set.Moves[act.Move.MoveSlot])
		return mv.Priority
	}
	ordered := append([]SideID(nil), movers...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if pa, pb := prio(a), prio(b); pa != pb {
			return pa > pb
		}
		sa, sb := e.active(state, a).Stats.Spe, e.active(state, b).Stats.Spe
		if sa != sb {
			return sa > sb
		}
		return r.IntN(2) == 0
	})
	return ordered
}

// process drena la cola en PhaseResolving. Cuando se vacía, decide qué sigue
// (forced switch por faint, fin de batalla, o próximo turno) en advance.
func (e *Engine) process(state *State, r rng.RNG) []Event {
	var evs []Event
	for {
		it, ok := state.Queue.Pop()
		if !ok {
			break
		}
		switch it.Kind {
		case QueueSwitch:
			evs = append(evs, e.executeSwitch(state, it.Side, it.Switch.TeamSlot)...)
		case QueueMove:
			evs = append(evs, e.executeMove(state, it.Side, it.MoveSlot, r)...)
		}
		// QueueRequestForcedSwitch / residuales: pasos posteriores (U-turn,
		// end-of-turn). El motor mínimo solo pausa por faints, al vaciar la cola.
	}
	return append(evs, e.advance(state)...)
}

// executeMove ejecuta el move de side. Si el atacante quedó debilitado antes de
// actuar (lo noqueó el rival más rápido), no hace nada.
func (e *Engine) executeMove(state *State, side SideID, slot int, r rng.RNG) []Event {
	attacker := e.active(state, side)
	if attacker.Fainted {
		return nil
	}

	foe := side.opp()
	defender := e.active(state, foe)
	moveID := attacker.Set.Moves[slot]

	attacker.PP[slot]--
	attacker.LastMoveUsed = moveID
	foeSlot := state.Sides[foe].Active
	evs := []Event{{Kind: EventMoveUsed, Side: side, Slot: state.Sides[side].Active, MoveID: moveID}}

	move, ok := e.Dex.Move(e.Rules.Gen, moveID)
	if !ok || move.Category == pokemon.CategoryStatus || move.Power <= 0 {
		return evs // move de estado o desconocido: no-op (efectos en pasos posteriores)
	}

	if !e.rollHit(move, attacker, defender, r) {
		return append(evs, Event{Kind: EventMiss, Side: side, MoveID: moveID, Reason: "miss"})
	}

	eff := e.effectiveness(move, defender)
	if eff == 0 {
		return append(evs, Event{Kind: EventImmune, Side: foe, Slot: foeSlot, MoveID: moveID})
	}

	crit := e.rollCrit(r)
	dmg := e.calcDamage(attacker, defender, move, crit, eff, r)
	if dmg > defender.HP {
		dmg = defender.HP
	}
	defender.HP -= dmg

	if crit {
		evs = append(evs, Event{Kind: EventCrit, Side: foe, Slot: foeSlot})
	}
	switch {
	case eff > 1:
		evs = append(evs, Event{Kind: EventSuperEffective, Side: foe, Slot: foeSlot})
	case eff < 1:
		evs = append(evs, Event{Kind: EventNotVeryEffective, Side: foe, Slot: foeSlot})
	}
	evs = append(evs, Event{Kind: EventDamage, Side: foe, Slot: foeSlot, MoveID: moveID, Amount: dmg})

	if defender.HP <= 0 {
		defender.HP = 0
		defender.Fainted = true
		evs = append(evs, Event{Kind: EventFainted, Side: foe, Slot: foeSlot})
	}
	return evs
}

// executeSwitch cambia el activo de side al slot dado. Resetea los boosts del
// que entra (mecánica estándar). Emite switch-out solo si el saliente no está
// debilitado (en un forced switch por faint no hay salida que animar).
func (e *Engine) executeSwitch(state *State, side SideID, slot int) []Event {
	s := &state.Sides[side]
	var evs []Event
	if !s.Team[s.Active].Fainted {
		evs = append(evs, Event{Kind: EventSwitchOut, Side: side, Slot: s.Active})
	}
	s.Active = slot
	s.Team[slot].Boosts = Boosts{}
	return append(evs, Event{Kind: EventSwitchIn, Side: side, Slot: slot})
}

// advance se llama cuando la cola se vació. Decide la transición:
//   - algún activo debilitado sin reemplazo → fin de batalla (o empate).
//   - algún activo debilitado con reemplazo → PhaseAwaitingForcedSwitch.
//   - sin faints → próximo turno (PhaseAwaitingActions).
//
// (Los residuales de fin de turno —clima, status, leftovers— son el paso 6 y
// todavía no se ejecutan acá.)
func (e *Engine) advance(state *State) []Event {
	aLoses := e.active(state, SideA).Fainted && !hasReplacement(&state.Sides[SideA])
	bLoses := e.active(state, SideB).Fainted && !hasReplacement(&state.Sides[SideB])
	if aLoses || bLoses {
		return e.endBattle(state, aLoses, bLoses)
	}

	var pending []SideID
	var evs []Event
	for _, side := range []SideID{SideA, SideB} {
		if e.active(state, side).Fainted {
			pending = append(pending, side)
			evs = append(evs, Event{Kind: EventRequestForcedSwitch, Side: side})
		}
	}
	if len(pending) > 0 {
		state.PendingSwitches = pending
		state.Phase = PhaseAwaitingForcedSwitch
		return evs
	}

	state.Turn++
	state.Phase = PhaseAwaitingActions
	return append(evs, Event{Kind: EventTurnStarted, Amount: state.Turn})
}

// applyForcedSwitch maneja el Switch que un lado debe enviar en
// PhaseAwaitingForcedSwitch.
func (e *Engine) applyForcedSwitch(state *State, action Action) ([]Event, error) {
	side := action.Side
	if !contains(state.PendingSwitches, side) {
		return nil, ErrWrongSide
	}
	if action.Switch == nil {
		return nil, ErrInvalidSwitchSlot
	}
	if err := e.validateSwitchTarget(state, side, action.Switch.TeamSlot); err != nil {
		return nil, err
	}

	evs := e.executeSwitch(state, side, action.Switch.TeamSlot)
	state.PendingSwitches = remove(state.PendingSwitches, side)

	if len(state.PendingSwitches) > 0 {
		return evs, nil // falta el otro lado (doble KO)
	}

	// Todos los forced switches resueltos. El motor mínimo no pausa a mitad de
	// cola, así que la cola está vacía y simplemente avanzamos al próximo turno.
	// (Cuando exista U-turn, acá habría que reanudar process si la cola tiene
	// items pendientes.)
	return append(evs, e.advance(state)...), nil
}

// forfeit termina la batalla: gana el rival del que se rinde.
func (e *Engine) forfeit(state *State, side SideID) []Event {
	winner := side.opp()
	state.Winner = &winner
	state.Phase = PhaseEnded
	return []Event{{Kind: EventBattleEnded, Side: winner, Reason: "forfeit"}}
}

// endBattle fija el resultado cuando uno (o ambos) lados se quedaron sin
// Pokémon utilizables.
func (e *Engine) endBattle(state *State, aLoses, bLoses bool) []Event {
	state.Phase = PhaseEnded
	switch {
	case aLoses && bLoses:
		return []Event{{Kind: EventBattleEnded, Reason: "draw"}}
	case aLoses:
		w := SideB
		state.Winner = &w
		return []Event{{Kind: EventBattleEnded, Side: w}}
	default:
		w := SideA
		state.Winner = &w
		return []Event{{Kind: EventBattleEnded, Side: w}}
	}
}

// --- helpers ---

func (e *Engine) active(state *State, side SideID) *BattlePokemon {
	s := &state.Sides[side]
	return &s.Team[s.Active]
}

func (s SideID) opp() SideID {
	if s == SideA {
		return SideB
	}
	return SideA
}

func turnSeed(state *State) uint64 { return state.Seed + uint64(state.Turn) }

// hasReplacement indica si el lado tiene algún Pokémon vivo distinto del activo
// para mandar a la batalla.
func hasReplacement(s *Side) bool {
	for i := range s.Team {
		if i != s.Active && !s.Team[i].Empty && !s.Team[i].Fainted {
			return true
		}
	}
	return false
}

func contains(sides []SideID, side SideID) bool {
	for _, s := range sides {
		if s == side {
			return true
		}
	}
	return false
}

func remove(sides []SideID, side SideID) []SideID {
	out := sides[:0:0]
	for _, s := range sides {
		if s != side {
			out = append(out, s)
		}
	}
	return out
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
