package battle

// endOfTurn aplica los residuales de fin de turno, en orden: clima (daño),
// luego por cada activo en orden de velocidad sus efectos (leftovers, leech
// seed, status), y por último el descuento de duración del clima.
//
// No usa RNG (todo es determinístico). Cada efecto saltea a un Pokémon ya
// debilitado, y marca Fainted cuando corresponde; advance se encarga después de
// los forced switches o el fin de batalla.
//
// Pendiente para más adelante: que moves/abilities/items efectivamente
// inflijan status, clima o leech seed (movepool, paso 8). Acá solo está el
// motor que los resuelve a partir del estado.
func (e *Engine) endOfTurn(state *State) []Event {
	evs := e.weatherDamage(state)
	for _, side := range e.residualOrder(state) {
		evs = append(evs, e.leftoversResidual(state, side)...)
		evs = append(evs, e.leechSeedResidual(state, side)...)
		evs = append(evs, e.statusResidual(state, side)...)
	}
	return append(evs, e.tickWeather(state)...)
}

// residualOrder devuelve los lados en orden de velocidad del activo (más rápido
// primero; empate → SideA). Importa para decidir quién cae primero si dos
// efectos noquean en el mismo fin de turno.
func (e *Engine) residualOrder(state *State) [2]SideID {
	if e.active(state, SideA).Stats.Spe >= e.active(state, SideB).Stats.Spe {
		return [2]SideID{SideA, SideB}
	}
	return [2]SideID{SideB, SideA}
}

// weatherDamage aplica el daño de arena/granizo a ambos activos no inmunes.
// Sol/lluvia/nieve no hacen daño residual.
func (e *Engine) weatherDamage(state *State) []Event {
	w := state.Field.Weather
	if w != "sand" && w != "hail" {
		return nil
	}
	var evs []Event
	for _, side := range []SideID{SideA, SideB} {
		bp := e.active(state, side)
		if bp.Fainted || e.weatherImmune(bp, w) {
			continue
		}
		evs = append(evs, e.hurt(state, side, bp.MaxHP/16, w)...)
	}
	return evs
}

// weatherImmune indica si el Pokémon no recibe daño del clima por su tipo.
func (e *Engine) weatherImmune(bp *BattlePokemon, weather string) bool {
	for _, t := range e.typesOf(bp) {
		switch weather {
		case "sand":
			if t == "rock" || t == "ground" || t == "steel" {
				return true
			}
		case "hail":
			if t == "ice" {
				return true
			}
		}
	}
	return false
}

// tickWeather descuenta la duración del clima y lo termina al llegar a 0.
// WeatherTurns <= 0 con clima activo se trata como permanente (no descuenta).
func (e *Engine) tickWeather(state *State) []Event {
	if state.Field.Weather == "" || state.Field.WeatherTurns <= 0 {
		return nil
	}
	state.Field.WeatherTurns--
	if state.Field.WeatherTurns == 0 {
		ended := state.Field.Weather
		state.Field.Weather = ""
		return []Event{{Kind: EventWeatherEnded, Reason: ended}}
	}
	return nil
}

// leftoversResidual cura 1/16 del HP máximo si el activo lleva Leftovers.
func (e *Engine) leftoversResidual(state *State, side SideID) []Event {
	if !e.Rules.HasItems {
		return nil
	}
	bp := e.active(state, side)
	if bp.Fainted || bp.Set.ItemID != "leftovers" {
		return nil
	}
	return e.heal(state, side, bp.MaxHP/16, "leftovers")
}

// leechSeedResidual drena 1/8 del HP del Pokémon sembrado y cura al activo rival
// por lo efectivamente drenado.
func (e *Engine) leechSeedResidual(state *State, side SideID) []Event {
	bp := e.active(state, side)
	if bp.Fainted || bp.Volatiles["leechseed"] == nil {
		return nil
	}
	drain := bp.MaxHP / 8
	if drain > bp.HP {
		drain = bp.HP
	}
	evs := e.hurt(state, side, drain, "leechseed")

	opp := side.opp()
	if ob := e.active(state, opp); !ob.Fainted {
		evs = append(evs, e.heal(state, opp, drain, "leechseed")...)
	}
	return evs
}

// statusResidual aplica el daño de quemadura/veneno/tóxico al final del turno.
func (e *Engine) statusResidual(state *State, side SideID) []Event {
	bp := e.active(state, side)
	if bp.Fainted {
		return nil
	}
	switch bp.Status {
	case StatusBurn:
		div := 16 // Gen 7+: 1/16; antes 1/8.
		if e.Rules.Gen < 7 {
			div = 8
		}
		return e.hurt(state, side, bp.MaxHP/div, "brn")
	case StatusPoison:
		return e.hurt(state, side, bp.MaxHP/8, "psn")
	case StatusToxic:
		bp.StatusData.ToxicCount++
		return e.hurt(state, side, bp.MaxHP*bp.StatusData.ToxicCount/16, "tox")
	}
	return nil
}

// hurt aplica daño residual al activo de side (mínimo 1, sin pasar de 0) y emite
// el daño y el faint si corresponde. reason identifica la fuente (status, clima…).
func (e *Engine) hurt(state *State, side SideID, amount int, reason string) []Event {
	bp := e.active(state, side)
	slot := state.Sides[side].Active
	if amount < 1 {
		amount = 1
	}
	if amount > bp.HP {
		amount = bp.HP
	}
	bp.HP -= amount
	evs := []Event{{Kind: EventDamage, Side: side, Slot: slot, Amount: amount, Reason: reason}}
	if bp.HP <= 0 {
		bp.HP = 0
		bp.Fainted = true
		evs = append(evs, Event{Kind: EventFainted, Side: side, Slot: slot})
	}
	return evs
}

// heal cura al activo de side (sin pasar del HP máximo). No emite nada si ya
// está al máximo.
func (e *Engine) heal(state *State, side SideID, amount int, reason string) []Event {
	bp := e.active(state, side)
	slot := state.Sides[side].Active
	if bp.Fainted || bp.HP >= bp.MaxHP {
		return nil
	}
	if amount < 1 {
		amount = 1
	}
	if bp.HP+amount > bp.MaxHP {
		amount = bp.MaxHP - bp.HP
	}
	bp.HP += amount
	return []Event{{Kind: EventHeal, Side: side, Slot: slot, Amount: amount, Reason: reason}}
}
