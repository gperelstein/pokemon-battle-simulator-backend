package battle

import (
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gperelstein/pokemon-battle-simulator-backend/pkg/rng"
)

// calcDamage computa el daño de un move que pega, usando la fórmula moderna:
// nivel, poder, atk/def (según categoría, con boosts), crítico, roll aleatorio
// 85..100, STAB y efectividad de tipos. eff se pasa ya calculado (el llamador
// lo necesita para decidir inmunidad/eventos y así evitamos un lookup doble).
//
// Pendiente para pasos siguientes: clima, burn (-50% físico), items
// (Life Orb, Choice Band…), abilities, multi-hit. La efectividad y el STAB de
// tipos múltiples por Tera tampoco se modelan aún.
func (e *Engine) calcDamage(attacker, defender *BattlePokemon, move pokemon.Move, crit bool, eff float64, r rng.RNG) int {
	cat := e.resolveCategory(move)

	atkStat, atkStage := attacker.Stats.Atk, attacker.Boosts.Atk
	defStat, defStage := defender.Stats.Def, defender.Boosts.Def
	if cat == pokemon.CategorySpecial {
		atkStat, atkStage = attacker.Stats.SpA, attacker.Boosts.SpA
		defStat, defStage = defender.Stats.SpD, defender.Boosts.SpD
	}
	// El crítico ignora los boosts ofensivos negativos del atacante y los
	// defensivos positivos del defensor.
	if crit {
		if atkStage < 0 {
			atkStage = 0
		}
		if defStage > 0 {
			defStage = 0
		}
	}
	a := applyBoost(atkStat, atkStage)
	d := applyBoost(defStat, defStage)
	if d < 1 {
		d = 1
	}

	lvl := attacker.Set.Level
	dmg := (2*lvl/5+2)*move.Power*a/d/50 + 2

	if crit {
		num, den := critMultiplier(e.Rules.Gen)
		dmg = dmg * num / den
	}
	dmg = dmg * (85 + r.IntN(16)) / 100 // roll 0.85..1.00

	if e.hasSTAB(attacker, move) {
		dmg = dmg * 3 / 2
	}
	dmg = int(float64(dmg) * eff)

	if eff > 0 && dmg < 1 {
		dmg = 1 // un golpe efectivo siempre hace al menos 1
	}
	return dmg
}

// effectiveness es el multiplicador de tipo del move contra el defensor.
func (e *Engine) effectiveness(move pokemon.Move, defender *BattlePokemon) float64 {
	return e.Dex.TypeEffectiveness(e.Rules.Gen, move.Type, e.typesOf(defender))
}

// hasSTAB indica si el move comparte tipo con el atacante (Same Type Attack
// Bonus, ×1.5).
func (e *Engine) hasSTAB(attacker *BattlePokemon, move pokemon.Move) bool {
	for _, t := range e.typesOf(attacker) {
		if t == move.Type {
			return true
		}
	}
	return false
}

// typesOf devuelve los tipos del Pokémon (de la species en el dex). Cambios de
// tipo en batalla (Soak, Roost, Tera) llegarán más adelante.
func (e *Engine) typesOf(bp *BattlePokemon) []pokemon.Type {
	sp, ok := e.Dex.Species(e.Rules.Gen, bp.Set.SpeciesID)
	if !ok {
		return nil
	}
	return sp.Types
}

// resolveCategory devuelve la categoría efectiva del move. Con el split
// físico/especial (Gen 4+) es la del propio move; antes (Gen 1-3) los moves de
// daño son físicos o especiales según su TIPO.
func (e *Engine) resolveCategory(move pokemon.Move) pokemon.MoveCategory {
	if e.Rules.HasPhysSpecSplit || move.Category == pokemon.CategoryStatus {
		return move.Category
	}
	if physicalTypesPreSplit[move.Type] {
		return pokemon.CategoryPhysical
	}
	return pokemon.CategorySpecial
}

// rollCrit decide si el golpe es crítico. Tasa base por generación; los moves
// de alto ratio de crítico (Slash, Stone Edge…) todavía no se distinguen
// (no exportamos critRatio).
func (e *Engine) rollCrit(r rng.RNG) bool {
	denom := 16 // Gen 2-5
	if e.Rules.Gen >= 6 {
		denom = 24
	}
	return r.IntN(denom) == 0
}

// rollHit decide si el move acierta, según su accuracy y los stages de
// precisión/evasión. Accuracy 0 = nunca falla.
func (e *Engine) rollHit(move pokemon.Move, attacker, defender *BattlePokemon, r rng.RNG) bool {
	if move.Accuracy == 0 {
		return true
	}
	stage := clampStage(attacker.Boosts.Acc - defender.Boosts.Eva)
	acc := float64(move.Accuracy) * accuracyMultiplier(stage)
	return float64(r.IntN(100)) < acc
}

// --- helpers de fórmula (puros) ---

// applyBoost aplica un stage de stat (-6..+6) a un valor: +n → ×(2+n)/2,
// -n → ×2/(2+n).
func applyBoost(stat, stage int) int {
	stage = clampStage(stage)
	if stage >= 0 {
		return stat * (2 + stage) / 2
	}
	return stat * 2 / (2 - stage)
}

// accuracyMultiplier es la tabla de precisión/evasión: +n → (3+n)/3,
// -n → 3/(3-n).
func accuracyMultiplier(stage int) float64 {
	stage = clampStage(stage)
	if stage >= 0 {
		return float64(3+stage) / 3
	}
	return 3 / float64(3-stage)
}

func clampStage(stage int) int {
	if stage > 6 {
		return 6
	}
	if stage < -6 {
		return -6
	}
	return stage
}

func critMultiplier(genID int) (num, den int) {
	if genID >= 6 {
		return 3, 2 // ×1.5
	}
	return 2, 1 // ×2 (Gen 2-5)
}

// physicalTypesPreSplit son los tipos cuyos moves de daño eran físicos antes del
// split físico/especial (Gen 1-3).
var physicalTypesPreSplit = map[pokemon.Type]bool{
	"normal": true, "fighting": true, "flying": true, "poison": true,
	"ground": true, "rock": true, "bug": true, "ghost": true, "steel": true,
}
