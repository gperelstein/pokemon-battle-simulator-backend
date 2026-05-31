package battle

import "github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"

// Items del scope del paso 8. (Leftovers se resuelve en endofturn.go.)

// choiceItem unifica Choice Band/Specs/Scarf: ×1.5 a una stat y bloqueo del
// primer move usado. stat distingue cuál: atk (Band), spa (Specs), spe (Scarf).
type choiceItem struct {
	stat pokemon.StatKey
}

func (c choiceItem) modifyStat(_ *BattlePokemon, stat pokemon.StatKey, val int) int {
	if stat == c.stat {
		return val * 3 / 2
	}
	return val
}

func (choiceItem) locksChoice() {}

// lifeOrb: ×1.3 al daño de los moves que pegan, a costa de 1/10 del HP máximo de
// retroceso tras un move ofensivo.
type lifeOrb struct{}

func (lifeOrb) modifyDamage(_ *BattlePokemon, _ pokemon.Move, dmg int) int { return dmg * 13 / 10 }

func (lifeOrb) onAfterMoveSelf(c *effCtx, holder SideID) []Event {
	bp := c.e.active(c.state, holder)
	if bp.Fainted {
		return nil
	}
	recoil := bp.MaxHP / 10
	if recoil < 1 {
		recoil = 1
	}
	return c.e.hurt(c.state, holder, recoil, "lifeorb")
}

// focusSash: sobrevive con 1 HP a un golpe letal desde HP máximo; se consume.
type focusSash struct{}

func (focusSash) surviveLethal(h *BattlePokemon, dmg int) (int, bool) {
	if h.HP == h.MaxHP && dmg >= h.HP {
		return h.HP - 1, true
	}
	return dmg, false
}
