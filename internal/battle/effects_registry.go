package battle

import "github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"

// defaultEffects construye el Registry con el set curado del paso 8. Agregar un
// move/ability/item es una línea acá; no hay branching por id en el motor.
func defaultEffects() *Registry {
	r := newRegistry()

	// --- moves: status (efecto principal, chance 100) ---
	r.moves["willowisp"] = statusEffect{status: StatusBurn, chance: 100}
	r.moves["thunderwave"] = statusEffect{status: StatusParalyze, chance: 100}
	r.moves["toxic"] = statusEffect{status: StatusToxic, chance: 100}
	r.moves["poisonpowder"] = statusEffect{status: StatusPoison, chance: 100}
	r.moves["spore"] = statusEffect{status: StatusSleep, chance: 100}
	r.moves["sleeppowder"] = statusEffect{status: StatusSleep, chance: 100}
	r.moves["hypnosis"] = statusEffect{status: StatusSleep, chance: 100}

	// --- moves: boosts a sí mismo ---
	r.moves["swordsdance"] = boostEffect{target: targetSelf, boosts: Boosts{Atk: 2}}
	r.moves["nastyplot"] = boostEffect{target: targetSelf, boosts: Boosts{SpA: 2}}
	r.moves["calmmind"] = boostEffect{target: targetSelf, boosts: Boosts{SpA: 1, SpD: 1}}
	r.moves["bulkup"] = boostEffect{target: targetSelf, boosts: Boosts{Atk: 1, Def: 1}}
	r.moves["dragondance"] = boostEffect{target: targetSelf, boosts: Boosts{Atk: 1, Spe: 1}}
	r.moves["irondefense"] = boostEffect{target: targetSelf, boosts: Boosts{Def: 2}}
	r.moves["agility"] = boostEffect{target: targetSelf, boosts: Boosts{Spe: 2}}

	// --- moves: clima ---
	r.moves["sunnyday"] = weatherEffect{weather: "sun", turns: 5}
	r.moves["raindance"] = weatherEffect{weather: "rain", turns: 5}
	r.moves["sandstorm"] = weatherEffect{weather: "sand", turns: 5}
	r.moves["hail"] = weatherEffect{weather: "hail", turns: 5}
	r.moves["snowscape"] = weatherEffect{weather: "snow", turns: 5}

	// --- moves: volátiles ---
	r.moves["leechseed"] = volatileEffect{id: "leechseed"}

	// --- moves: secundarios (chance < 100) en moves de daño ---
	r.moves["flamethrower"] = statusEffect{status: StatusBurn, chance: 10}
	r.moves["fireblast"] = statusEffect{status: StatusBurn, chance: 10}
	r.moves["scald"] = statusEffect{status: StatusBurn, chance: 30}
	r.moves["icebeam"] = statusEffect{status: StatusFreeze, chance: 10}
	r.moves["icepunch"] = statusEffect{status: StatusFreeze, chance: 10}
	r.moves["nuzzle"] = statusEffect{status: StatusParalyze, chance: 100}

	// --- abilities ---
	r.abilities["levitate"] = levitate{}
	r.abilities["intimidate"] = intimidate{}
	r.abilities["sturdy"] = sturdy{}
	r.abilities["speedboost"] = speedBoost{}
	r.abilities["prankster"] = prankster{}

	// --- items ---
	r.items["choiceband"] = choiceItem{stat: pokemon.StatAtk}
	r.items["choicespecs"] = choiceItem{stat: pokemon.StatSpA}
	r.items["choicescarf"] = choiceItem{stat: pokemon.StatSpe}
	r.items["lifeorb"] = lifeOrb{}
	r.items["focussash"] = focusSash{}

	return r
}
