package battle

import (
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// computeStats calcula los stats finales de un set al entrar a la batalla.
//
// Usa la fórmula moderna (Gen 3+): IVs 0..31, EVs 0..255 y modificador de
// naturaleza ±10%. Gen 1/2 (DVs + stat experience, sin naturaleza) usan otra
// fórmula y quedan pendientes — el plan dice "Gen 1 puede esperar"; mientras
// tanto se aplica la moderna también para gens tempranas (aproximación).
//
// TODO(gen1-2): fórmula con DVs/stat-exp cuando se implemente esa generación.
func computeStats(base pokemon.Stats, set pokemon.Pokemon, rules gen.Rules) (maxHP int, stats pokemon.Stats) {
	lvl := set.Level

	// HP tiene su propia fórmula (suma Level + 10 en vez de + 5, sin naturaleza).
	hp := (2*base.HP+set.IVs.HP+set.EVs.HP/4)*lvl/100 + lvl + 10

	calc := func(b, iv, ev int, mod float64) int {
		v := (2*b+iv+ev/4)*lvl/100 + 5
		return int(float64(v) * mod) // trunca = floor para valores positivos
	}

	plus, minus := pokemon.StatKey(""), pokemon.StatKey("")
	if rules.HasNatures {
		plus, minus = natureEffect(set.Nature)
	}
	mod := func(k pokemon.StatKey) float64 {
		switch {
		case k == plus && k != minus:
			return 1.1
		case k == minus && k != plus:
			return 0.9
		default:
			return 1.0
		}
	}

	stats = pokemon.Stats{
		HP:  hp,
		Atk: calc(base.Atk, set.IVs.Atk, set.EVs.Atk, mod(pokemon.StatAtk)),
		Def: calc(base.Def, set.IVs.Def, set.EVs.Def, mod(pokemon.StatDef)),
		SpA: calc(base.SpA, set.IVs.SpA, set.EVs.SpA, mod(pokemon.StatSpA)),
		SpD: calc(base.SpD, set.IVs.SpD, set.EVs.SpD, mod(pokemon.StatSpD)),
		Spe: calc(base.Spe, set.IVs.Spe, set.EVs.Spe, mod(pokemon.StatSpe)),
	}
	return hp, stats
}

// natureEffect devuelve (statAumentada, statDisminuida) de una naturaleza. Para
// las 5 naturalezas neutras ambas coinciden (efecto neto 1.0). Naturaleza vacía
// o desconocida → neutral.
func natureEffect(n pokemon.Nature) (plus, minus pokemon.StatKey) {
	e, ok := natures[n]
	if !ok {
		return "", ""
	}
	return e[0], e[1]
}

// natures mapea cada naturaleza a [statAumentada, statDisminuida].
var natures = map[pokemon.Nature][2]pokemon.StatKey{
	"hardy":   {pokemon.StatAtk, pokemon.StatAtk}, // neutras: plus == minus
	"docile":  {pokemon.StatDef, pokemon.StatDef},
	"serious": {pokemon.StatSpe, pokemon.StatSpe},
	"bashful": {pokemon.StatSpA, pokemon.StatSpA},
	"quirky":  {pokemon.StatSpD, pokemon.StatSpD},

	"lonely":  {pokemon.StatAtk, pokemon.StatDef},
	"brave":   {pokemon.StatAtk, pokemon.StatSpe},
	"adamant": {pokemon.StatAtk, pokemon.StatSpA},
	"naughty": {pokemon.StatAtk, pokemon.StatSpD},

	"bold":    {pokemon.StatDef, pokemon.StatAtk},
	"relaxed": {pokemon.StatDef, pokemon.StatSpe},
	"impish":  {pokemon.StatDef, pokemon.StatSpA},
	"lax":     {pokemon.StatDef, pokemon.StatSpD},

	"timid": {pokemon.StatSpe, pokemon.StatAtk},
	"hasty": {pokemon.StatSpe, pokemon.StatDef},
	"jolly": {pokemon.StatSpe, pokemon.StatSpA},
	"naive": {pokemon.StatSpe, pokemon.StatSpD},

	"modest": {pokemon.StatSpA, pokemon.StatAtk},
	"mild":   {pokemon.StatSpA, pokemon.StatDef},
	"quiet":  {pokemon.StatSpA, pokemon.StatSpe},
	"rash":   {pokemon.StatSpA, pokemon.StatSpD},

	"calm":    {pokemon.StatSpD, pokemon.StatAtk},
	"gentle":  {pokemon.StatSpD, pokemon.StatDef},
	"sassy":   {pokemon.StatSpD, pokemon.StatSpe},
	"careful": {pokemon.StatSpD, pokemon.StatSpA},
}
