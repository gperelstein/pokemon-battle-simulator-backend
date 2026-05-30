package battle

import (
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/pkg/rng"
)

// dummyDamage es el daño "puro" del motor mínimo (paso 3): usa el núcleo de la
// fórmula moderna (nivel, poder, atk/def según categoría) con el roll aleatorio
// 85..100, pero SIN type chart, STAB, críticos, clima ni abilities/items.
//
// El paso 4 reemplaza esto por la fórmula real con efectividad de tipos y STAB.
func dummyDamage(attacker, defender *BattlePokemon, move pokemon.Move, r rng.RNG) int {
	if move.Power <= 0 {
		return 0 // moves de estado / sin poder: no hacen daño
	}

	atk, def := attacker.Stats.Atk, defender.Stats.Def
	if move.Category == pokemon.CategorySpecial {
		atk, def = attacker.Stats.SpA, defender.Stats.SpD
	}
	if def < 1 {
		def = 1
	}

	lvl := attacker.Set.Level
	base := (2*lvl/5+2)*move.Power*atk/def/50 + 2
	base = base * (85 + r.IntN(16)) / 100 // roll 0.85..1.00
	if base < 1 {
		base = 1
	}
	return base
}
