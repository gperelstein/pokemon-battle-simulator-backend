package battle

import "github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"

// Abilities del scope del paso 8. Cada una implementa solo los hooks que usa.

// levitate: inmunidad total a los moves de tipo ground. (Mold Breaker, Iron
// Ball, Gravity y Roost quedan fuera de scope.)
type levitate struct{}

func (levitate) immuneToMove(_ *BattlePokemon, t pokemon.Type) bool { return t == "ground" }

// intimidate: al entrar, baja 1 stage el Ataque del rival activo.
type intimidate struct{}

func (intimidate) onSwitchIn(c *effCtx, holder SideID) []Event {
	foe := holder.opp()
	fb := c.e.active(c.state, foe)
	if fb.Fainted || fb.Empty {
		return nil
	}
	evs := []Event{{Kind: EventAbilityActivated, Side: holder, Slot: c.state.Sides[holder].Active, Reason: "intimidate"}}
	return append(evs, c.e.applyBoosts(c.state, foe, Boosts{Atk: -1})...)
}

// sturdy: sobrevive con 1 HP a un golpe que noquearía desde HP máximo.
type sturdy struct{}

func (sturdy) surviveLethal(h *BattlePokemon, dmg int) (int, bool) {
	if h.HP == h.MaxHP && dmg >= h.HP {
		return h.HP - 1, true
	}
	return dmg, false
}

// speedBoost: al final del turno sube 1 stage de Velocidad.
type speedBoost struct{}

func (speedBoost) onResidual(c *effCtx, holder SideID) []Event {
	if c.e.active(c.state, holder).Boosts.Spe >= 6 {
		return nil
	}
	evs := []Event{{Kind: EventAbilityActivated, Side: holder, Slot: c.state.Sides[holder].Active, Reason: "speedboost"}}
	return append(evs, c.e.applyBoosts(c.state, holder, Boosts{Spe: 1})...)
}

// prankster: +1 de prioridad a los moves de estado (gen 5+). La inmunidad de los
// tipo Dark a los status con prioridad de Prankster (gen 7+) queda como TODO.
type prankster struct{}

func (prankster) modifyPriority(_ *BattlePokemon, move pokemon.Move, prio int) int {
	if move.Category == pokemon.CategoryStatus {
		return prio + 1
	}
	return prio
}
