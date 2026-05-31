package battle

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gperelstein/pokemon-battle-simulator-backend/pkg/rng"
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
	// Effects es el registro de efectos (moves/abilities/items). Un id no
	// registrado cae al comportamiento por defecto (daño puro / no-op).
	Effects *Registry
}

// New construye un Engine para una generación concreta, con el set de efectos
// por defecto (ver defaultEffects).
func New(d dex.Dex, rules gen.Rules) *Engine {
	return &Engine{Dex: d, Rules: rules, Effects: defaultEffects()}
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
	// Entry-abilities (Intimidate…) en orden de velocidad. No usan RNG, pero se
	// pasa uno derivado del seed para mantener la firma uniforme.
	r := rng.New(state.Seed)
	for _, side := range e.residualOrder(state) {
		evs = append(evs, e.abilityOnSwitchIn(state, side, r)...)
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
		// Choice lock: si el item bloqueó un slot, solo se puede repetir ese move.
		if locked, ok := active.Volatiles["choicelock"].(int); ok && locked != action.Move.MoveSlot {
			return ErrChoiceLocked
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
		bp := e.active(state, side)
		mv, _ := e.Dex.Move(e.Rules.Gen, bp.Set.Moves[state.PendingActions[side].Move.MoveSlot])
		p := mv.Priority
		// Prankster y similares pueden modificar la prioridad.
		if a, ok := e.abilityOf(bp).(abilityPriority); ok {
			p = a.modifyPriority(bp, mv, p)
		}
		return p
	}
	ordered := append([]SideID(nil), movers...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if pa, pb := prio(a), prio(b); pa != pb {
			return pa > pb
		}
		sa, sb := e.effectiveSpeed(e.active(state, a)), e.effectiveSpeed(e.active(state, b))
		if sa != sb {
			return sa > sb
		}
		return r.IntN(2) == 0
	})
	return ordered
}

// process drena la cola en PhaseResolving. Si encuentra un
// QueueRequestForcedSwitch (lo empuja un move con selfSwitch, p.ej. U-turn),
// pausa a PhaseAwaitingForcedSwitch dejando el resto de la cola intacto. Cuando
// la cola se vacía, decide qué sigue (forced switch por faint, fin de batalla,
// o próximo turno) en advance.
func (e *Engine) process(state *State, r rng.RNG) []Event {
	var evs []Event
	for {
		it, ok := state.Queue.Pop()
		if !ok {
			break
		}
		switch it.Kind {
		case QueueSwitch:
			evs = append(evs, e.executeSwitch(state, it.Side, it.Switch.TeamSlot, "", r)...)
		case QueueMove:
			evs = append(evs, e.executeMove(state, it.Side, it.MoveSlot, r)...)
		case QueueRequestForcedSwitch:
			// Pausa mid-turn: el atacante de un selfSwitch (U-turn/Volt Switch)
			// debe elegir relevo. La cola queda intacta y se reanuda al recibirlo.
			state.PendingSwitches = append(state.PendingSwitches, it.Side)
			state.Phase = PhaseAwaitingForcedSwitch
			return append(evs, Event{Kind: EventRequestForcedSwitch, Side: it.Side})
		}
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
	foeSlot := state.Sides[foe].Active

	// Status que impide actuar (sueño/congelado/parálisis). No consume PP ni
	// emite "move usado"; sí puede emitir despertar/descongelar.
	canMove, evs := e.beforeMoveStatus(state, side, r)
	if !canMove {
		return evs
	}

	attacker.PP[slot]--
	attacker.LastMoveUsed = moveID
	evs = append(evs, Event{Kind: EventMoveUsed, Side: side, Slot: state.Sides[side].Active, MoveID: moveID})

	move, ok := e.Dex.Move(e.Rules.Gen, moveID)
	if !ok {
		return evs // move desconocido: no-op
	}

	// Choice lock: usar el move bloquea al portador a este slot (incluso si falla).
	if _, locks := e.itemOf(attacker).(choiceLocker); locks {
		if attacker.Volatiles == nil {
			attacker.Volatiles = map[string]any{}
		}
		attacker.Volatiles["choicelock"] = slot
	}

	if !e.rollHit(move, attacker, defender, r) {
		return append(evs, Event{Kind: EventMiss, Side: side, MoveID: moveID, Reason: "miss"})
	}

	isDamaging := move.Category != pokemon.CategoryStatus && move.Power > 0
	if isDamaging {
		eff := e.effectiveness(move, defender)
		// Inmune por tipo o por ability (Levitate): el move falla por completo.
		if eff == 0 || e.abilityImmune(defender, move.Type) {
			return append(evs, Event{Kind: EventImmune, Side: foe, Slot: foeSlot, MoveID: moveID})
		}
		evs = append(evs, e.applyDamage(state, side, foe, move, eff, r)...)
	}

	// Efecto del move (status/boost/clima/volátil principal, o secundario tras el
	// daño). Cada efecto resuelve su propia elegibilidad y saltea targets caídos.
	if me := e.Effects.moves[moveID]; me != nil {
		mc := &moveCtx{e: e, state: state, rng: r, userSide: side, targetSide: foe, move: move}
		evs = append(evs, me.onHit(mc)...)
	}

	// Items que reaccionan tras un move ofensivo propio (Life Orb: retroceso).
	if isDamaging {
		if it, ok := e.itemOf(attacker).(afterMoveSelf); ok {
			evs = append(evs, it.onAfterMoveSelf(&effCtx{e: e, state: state, rng: r}, side)...)
		}
	}

	// Efectos de cambio forzado, después del daño:
	if move.ForceSwitch {
		evs = append(evs, e.dragOut(state, foe, r)...)
	}
	if move.SelfSwitch != "" && !attacker.Fainted && hasReplacement(&state.Sides[side]) {
		// El atacante elige relevo: se pausa cuando process saque este item.
		// (copyvolatile de Baton Pass —transferir boosts— queda pendiente.)
		state.Queue.PushFront(QueueItem{Kind: QueueRequestForcedSwitch, Side: side})
	}
	return evs
}

// beforeMoveStatus resuelve el status que puede impedir actuar al activo de side
// (sueño, congelado, parálisis). Devuelve si puede moverse y los eventos
// asociados. Solo consume RNG cuando hay un status que lo requiere, para no
// perturbar la secuencia de turnos sin status.
func (e *Engine) beforeMoveStatus(state *State, side SideID, r rng.RNG) (bool, []Event) {
	bp := e.active(state, side)
	slot := state.Sides[side].Active
	switch bp.Status {
	case StatusSleep:
		bp.StatusData.SleepTurns--
		if bp.StatusData.SleepTurns <= 0 {
			bp.StatusData.SleepTurns = 0
			bp.Status = StatusNone
			return true, []Event{{Kind: EventStatusCured, Side: side, Slot: slot, Status: "slp"}}
		}
		return false, []Event{{Kind: EventStatusInflicted, Side: side, Slot: slot, Status: "slp", Reason: "asleep"}}
	case StatusFreeze:
		if r.IntN(100) < 20 { // 20% de descongelarse y poder actuar
			bp.Status = StatusNone
			return true, []Event{{Kind: EventStatusCured, Side: side, Slot: slot, Status: "frz"}}
		}
		return false, []Event{{Kind: EventStatusInflicted, Side: side, Slot: slot, Status: "frz", Reason: "frozen"}}
	case StatusParalyze:
		if r.IntN(100) < 25 { // 25% de parálisis total
			return false, []Event{{Kind: EventStatusInflicted, Side: side, Slot: slot, Status: "par", Reason: "fullpara"}}
		}
	}
	return true, nil
}

// applyDamage calcula y aplica el daño de un move que pega, y devuelve los
// eventos (crit, efectividad, daño, faint). eff ya viene calculado (>0).
func (e *Engine) applyDamage(state *State, side, foe SideID, move pokemon.Move, eff float64, r rng.RNG) []Event {
	attacker := e.active(state, side)
	defender := e.active(state, foe)
	foeSlot := state.Sides[foe].Active

	crit := e.rollCrit(r)
	dmg := e.calcDamage(attacker, defender, move, crit, eff, r)

	// Sturdy / Focus Sash: sobrevivir con 1 HP a un golpe letal desde HP máximo.
	var guardEvs []Event
	if dmg >= defender.HP {
		dmg, guardEvs = e.applyLethalGuard(state, foe, dmg)
	}
	if dmg > defender.HP {
		dmg = defender.HP
	}
	defender.HP -= dmg

	var evs []Event
	if crit {
		evs = append(evs, Event{Kind: EventCrit, Side: foe, Slot: foeSlot})
	}
	switch {
	case eff > 1:
		evs = append(evs, Event{Kind: EventSuperEffective, Side: foe, Slot: foeSlot})
	case eff < 1:
		evs = append(evs, Event{Kind: EventNotVeryEffective, Side: foe, Slot: foeSlot})
	}
	evs = append(evs, Event{Kind: EventDamage, Side: foe, Slot: foeSlot, MoveID: move.ID, Amount: dmg})
	evs = append(evs, guardEvs...) // activación de Sturdy/Focus Sash, tras el daño

	if defender.HP <= 0 {
		defender.HP = 0
		defender.Fainted = true
		evs = append(evs, Event{Kind: EventFainted, Side: foe, Slot: foeSlot})
	}
	return evs
}

// executeSwitch cambia el activo de side al slot dado. Resetea los boosts del
// que entra (mecánica estándar). Emite switch-out solo si el saliente no está
// debilitado (en un forced switch por faint no hay salida que animar). reason
// queda en los eventos ("" voluntario/replacement, "drag" para Roar/Dragon Tail).
func (e *Engine) executeSwitch(state *State, side SideID, slot int, reason string, r rng.RNG) []Event {
	s := &state.Sides[side]
	var evs []Event
	if !s.Team[s.Active].Fainted {
		evs = append(evs, Event{Kind: EventSwitchOut, Side: side, Slot: s.Active, Reason: reason})
	}
	// El que sale pierde el bloqueo de Choice (se re-evalúa si vuelve a entrar).
	delete(s.Team[s.Active].Volatiles, "choicelock")
	s.Active = slot
	s.Team[slot].Boosts = Boosts{}
	if s.Team[slot].Volatiles == nil {
		s.Team[slot].Volatiles = map[string]any{}
	}
	evs = append(evs, Event{Kind: EventSwitchIn, Side: side, Slot: slot, Reason: reason})
	// Entry-ability del que entra (Intimidate…).
	return append(evs, e.abilityOnSwitchIn(state, side, r)...)
}

// dragOut saca al activo de side a un Pokémon vivo del banco elegido al azar
// (Roar, Whirlwind, Dragon Tail, Circle Throw). No hace nada si el activo está
// debilitado (lo resuelve el flujo de faint) o si no hay reemplazo.
func (e *Engine) dragOut(state *State, side SideID, r rng.RNG) []Event {
	s := &state.Sides[side]
	if s.Team[s.Active].Fainted {
		return nil
	}
	var choices []int
	for i := range s.Team {
		if i != s.Active && !s.Team[i].Empty && !s.Team[i].Fainted {
			choices = append(choices, i)
		}
	}
	if len(choices) == 0 {
		return nil
	}
	pick := choices[r.IntN(len(choices))]
	return e.executeSwitch(state, side, pick, "drag", r)
}

// advance se llama cuando la cola se vació. Resuelve la secuencia de fin de
// turno por etapas, pausando en PhaseAwaitingForcedSwitch cada vez que hace
// falta input y reanudándose (vía applyForcedSwitch) hasta llegar al próximo
// turno:
//
//  1. faints (de moves o de residuales) sin reemplazo → fin de batalla;
//     con reemplazo → forced switch.
//  2. sin faints pendientes y residuales aún no aplicados → end-of-turn
//     (clima, status, leftovers, leech seed), que puede volver a causar faints.
//  3. residuales ya aplicados y sin faints → próximo turno.
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

	// Sin faints pendientes: aplicar residuales de fin de turno una sola vez.
	if !state.EotDone {
		state.EotDone = true
		state.Phase = PhaseEndOfTurn
		evs = append(evs, e.endOfTurn(state)...)
		// Los residuales pueden haber noqueado a alguien: re-evaluar.
		return append(evs, e.advance(state)...)
	}

	state.Turn++
	state.ResumeCount = 0
	state.EotDone = false
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

	evs := e.executeSwitch(state, side, action.Switch.TeamSlot, "", rng.New(turnSeed(state)))
	state.PendingSwitches = remove(state.PendingSwitches, side)

	if len(state.PendingSwitches) > 0 {
		return evs, nil // falta el otro lado (doble KO o ambos con selfSwitch)
	}

	// Todos los forced switches resueltos. Si la cola todavía tiene items, fue
	// una pausa mid-turn (U-turn): se reanuda el drenado con un RNG nuevo e
	// independiente. Si está vacía, fue un faint de fin de turno: avanza.
	if !state.Queue.Empty() {
		state.ResumeCount++
		state.Phase = PhaseResolving
		return append(evs, e.process(state, rng.New(turnSeed(state)))...), nil
	}
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

// turnSeed deriva la semilla del segmento de resolución actual a partir de
// (Seed, Turn, ResumeCount). El multiplicador primo evita que turnos y
// reanudaciones colisionen; rng.New mezcla además con el número áureo.
func turnSeed(state *State) uint64 {
	return state.Seed + uint64(state.Turn)*0x100000001b3 + uint64(state.ResumeCount)
}

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
	ErrChoiceLocked      = errors.New("battle: locked into a move by a Choice item")
)
