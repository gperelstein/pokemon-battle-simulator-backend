package battle

import (
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gperelstein/pokemon-battle-simulator-backend/pkg/rng"
)

// Sistema de efectos (paso 8).
//
// Diseño: el comportamiento de moves/abilities/items vive en *valores de efecto*
// registrados por id (mismo id que el dataset de Showdown) en un Registry. El
// motor los despacha con un lookup en mapa + type-assertion sobre interfaces de
// hook pequeñas y opcionales — nunca con un switch gigante por id. La variedad
// de moves "data-shaped" (status, boosts, clima, volátiles) se cubre con unos
// pocos tipos componibles (ver effects_moves.go), de modo que registrar un move
// nuevo es una línea de tabla, no código nuevo.
//
// El paquete effect/ original se plegó acá: los hooks necesitan los tipos de
// battle (*State, *BattlePokemon, Event…) y mantenerlos en un subpaquete
// generaba un ciclo de imports.

// Registry contiene las implementaciones por id. Lookups O(1); un id ausente
// cae al comportamiento por defecto (daño puro / no-op).
//
// Los valores de ability/item se guardan como `any` porque cada uno implementa
// solo el subconjunto de hooks que le toca; el motor hace la type-assertion en
// cada call-site.
type Registry struct {
	moves     map[string]moveEffect
	abilities map[string]any
	items     map[string]any
}

func newRegistry() *Registry {
	return &Registry{
		moves:     map[string]moveEffect{},
		abilities: map[string]any{},
		items:     map[string]any{},
	}
}

// --- contextos pasados a los hooks ---

// moveCtx es el contexto de ejecución del efecto de un move.
type moveCtx struct {
	e          *Engine
	state      *State
	rng        rng.RNG
	userSide   SideID
	targetSide SideID
	move       pokemon.Move
}

func (c *moveCtx) user() *BattlePokemon   { return c.e.active(c.state, c.userSide) }
func (c *moveCtx) target() *BattlePokemon { return c.e.active(c.state, c.targetSide) }

// effCtx es el contexto de los hooks de ability/item (no atados a un move).
type effCtx struct {
	e     *Engine
	state *State
	rng   rng.RNG
}

// --- interfaces de hook ---

// moveEffect es el efecto de un move más allá del daño: status, boosts, clima,
// volátiles. Se invoca en executeMove tras el daño (o como efecto principal de
// un status move). Cada efecto se ocupa de su propia elegibilidad (tipo del
// target, status ya presente, etc.).
type moveEffect interface {
	onHit(c *moveCtx) []Event
}

// Hooks de ability (todos opcionales; el motor hace type-assert):
type abilitySwitchIn interface {
	onSwitchIn(c *effCtx, holder SideID) []Event // Intimidate
}
type abilityResidual interface {
	onResidual(c *effCtx, holder SideID) []Event // Speed Boost
}
type abilityPriority interface {
	modifyPriority(holder *BattlePokemon, move pokemon.Move, prio int) int // Prankster
}
type abilityImmunity interface {
	immuneToMove(holder *BattlePokemon, moveType pokemon.Type) bool // Levitate
}

// survivor lo implementan ability (Sturdy) e item (Focus Sash): reduce un golpe
// letal recibido a HP máximo a dejar 1 HP. Devuelve el daño ajustado y si actuó.
type survivor interface {
	surviveLethal(holder *BattlePokemon, dmg int) (int, bool)
}

// Hooks de item:
type statModifier interface {
	modifyStat(holder *BattlePokemon, stat pokemon.StatKey, val int) int // Choice Band/Specs/Scarf
}
type damageModifier interface {
	modifyDamage(holder *BattlePokemon, move pokemon.Move, dmg int) int // Life Orb
}
type afterMoveSelf interface {
	onAfterMoveSelf(c *effCtx, holder SideID) []Event // Life Orb recoil
}
type choiceLocker interface {
	locksChoice() // marca: el item bloquea al primer move usado (Choice)
}

// --- accesores de efecto del motor ---

// abilityOf devuelve el valor de efecto de la ability elegida del Pokémon, o nil
// si la gen no tiene abilities, el set no eligió una, o no está registrada.
func (e *Engine) abilityOf(bp *BattlePokemon) any {
	if !e.Rules.HasAbilities || bp.Set.AbilityID == "" {
		return nil
	}
	return e.Effects.abilities[bp.Set.AbilityID]
}

// itemOf devuelve el valor de efecto del item del Pokémon, o nil.
func (e *Engine) itemOf(bp *BattlePokemon) any {
	if !e.Rules.HasItems || bp.Set.ItemID == "" {
		return nil
	}
	return e.Effects.items[bp.Set.ItemID]
}

// --- helpers de aplicación de efectos (reutilizados por los valores de efecto) ---

// canApplyStatus indica si se le puede infligir st al activo de side: no debe
// tener ya un status y no debe ser inmune por tipo (gen-dependiente para par).
func (e *Engine) canApplyStatus(state *State, side SideID, st StatusCondition) bool {
	bp := e.active(state, side)
	if bp.Status != StatusNone {
		return false
	}
	for _, t := range e.typesOf(bp) {
		switch st {
		case StatusBurn:
			if t == "fire" {
				return false
			}
		case StatusParalyze:
			if e.Rules.Gen >= 6 && t == "electric" {
				return false
			}
		case StatusPoison, StatusToxic:
			if t == "poison" || t == "steel" {
				return false
			}
		case StatusFreeze:
			if t == "ice" {
				return false
			}
		}
	}
	return true
}

// inflictStatus setea el status (asume canApplyStatus ya true). Para sleep usa
// 1..3 turnos; para toxic resetea el contador (el EOT lo incrementa).
func (e *Engine) inflictStatus(state *State, side SideID, st StatusCondition, r rng.RNG) []Event {
	bp := e.active(state, side)
	bp.Status = st
	switch st {
	case StatusToxic:
		bp.StatusData.ToxicCount = 0
	case StatusSleep:
		bp.StatusData.SleepTurns = 1 + r.IntN(3)
	}
	return []Event{{Kind: EventStatusInflicted, Side: side, Slot: state.Sides[side].Active, Status: string(st)}}
}

// applyBoosts aplica un vector de stages al activo de side (clamp -6..+6) y emite
// un EventBoostChanged por stat efectivamente cambiada.
func (e *Engine) applyBoosts(state *State, side SideID, b Boosts) []Event {
	bp := e.active(state, side)
	slot := state.Sides[side].Active
	var evs []Event
	apply := func(cur *int, delta int, name string) {
		if delta == 0 {
			return
		}
		before := *cur
		n := clampStage(before + delta)
		if n == before {
			return // ya estaba al tope
		}
		*cur = n
		evs = append(evs, Event{Kind: EventBoostChanged, Side: side, Slot: slot, Stat: name, Amount: n - before})
	}
	apply(&bp.Boosts.Atk, b.Atk, "atk")
	apply(&bp.Boosts.Def, b.Def, "def")
	apply(&bp.Boosts.SpA, b.SpA, "spa")
	apply(&bp.Boosts.SpD, b.SpD, "spd")
	apply(&bp.Boosts.Spe, b.Spe, "spe")
	apply(&bp.Boosts.Acc, b.Acc, "accuracy")
	apply(&bp.Boosts.Eva, b.Eva, "evasion")
	return evs
}

// effectiveSpeed es la velocidad real del activo: stage de boosts, Choice Scarf
// y la baja por parálisis (½ gen7+, ¼ antes). Usada para ordenar y para
// residuales.
func (e *Engine) effectiveSpeed(bp *BattlePokemon) int {
	spe := applyBoost(bp.Stats.Spe, bp.Boosts.Spe)
	spe = e.modifyStatValue(bp, pokemon.StatSpe, spe)
	if bp.Status == StatusParalyze {
		div := 2
		if e.Rules.Gen < 7 {
			div = 4
		}
		spe /= div
	}
	return spe
}

// modifyStatValue aplica los modificadores de item al valor de una stat (Choice
// Band/Specs sobre atk/spa, Choice Scarf sobre spe).
func (e *Engine) modifyStatValue(bp *BattlePokemon, stat pokemon.StatKey, val int) int {
	if m, ok := e.itemOf(bp).(statModifier); ok {
		val = m.modifyStat(bp, stat, val)
	}
	return val
}

// modifyDamageValue aplica los modificadores de daño de item (Life Orb).
func (e *Engine) modifyDamageValue(bp *BattlePokemon, move pokemon.Move, dmg int) int {
	if m, ok := e.itemOf(bp).(damageModifier); ok {
		dmg = m.modifyDamage(bp, move, dmg)
	}
	return dmg
}

// abilityImmune indica si la ability del defensor lo hace inmune a moveType
// (Levitate vs ground).
func (e *Engine) abilityImmune(defender *BattlePokemon, moveType pokemon.Type) bool {
	if a, ok := e.abilityOf(defender).(abilityImmunity); ok {
		return a.immuneToMove(defender, moveType)
	}
	return false
}

// applyLethalGuard da la chance a la ability (Sturdy) y luego al item (Focus
// Sash) de sobrevivir un golpe letal desde HP máximo. Devuelve el daño ajustado
// y el evento de activación (consumiendo el item si fue el item).
func (e *Engine) applyLethalGuard(state *State, side SideID, dmg int) (int, []Event) {
	defender := e.active(state, side)
	slot := state.Sides[side].Active
	if a, ok := e.abilityOf(defender).(survivor); ok {
		if nd, saved := a.surviveLethal(defender, dmg); saved {
			return nd, []Event{{Kind: EventAbilityActivated, Side: side, Slot: slot, Reason: defender.Set.AbilityID}}
		}
	}
	if it, ok := e.itemOf(defender).(survivor); ok {
		if nd, saved := it.surviveLethal(defender, dmg); saved {
			consumed := defender.Set.ItemID
			defender.Set.ItemID = "" // item consumido
			return nd, []Event{{Kind: EventItemConsumed, Side: side, Slot: slot, Reason: consumed}}
		}
	}
	return dmg, nil
}

// abilityOnSwitchIn dispara la entry-ability del activo de side (Intimidate).
func (e *Engine) abilityOnSwitchIn(state *State, side SideID, r rng.RNG) []Event {
	bp := e.active(state, side)
	if bp.Fainted || bp.Empty {
		return nil
	}
	if a, ok := e.abilityOf(bp).(abilitySwitchIn); ok {
		return a.onSwitchIn(&effCtx{e: e, state: state, rng: r}, side)
	}
	return nil
}

// abilityResidual dispara la ability de fin de turno del activo de side
// (Speed Boost).
func (e *Engine) abilityResidual(state *State, side SideID, r rng.RNG) []Event {
	bp := e.active(state, side)
	if bp.Fainted {
		return nil
	}
	if a, ok := e.abilityOf(bp).(abilityResidual); ok {
		return a.onResidual(&effCtx{e: e, state: state, rng: r}, side)
	}
	return nil
}

// rollChance devuelve true con probabilidad chance/100. chance<=0 o >=100 nunca
// consume RNG (efecto primario "siempre").
func rollChance(r rng.RNG, chance int) bool {
	if chance <= 0 || chance >= 100 {
		return true
	}
	return r.IntN(100) < chance
}
