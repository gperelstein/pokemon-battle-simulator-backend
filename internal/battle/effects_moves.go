package battle

// Valores de efecto de moves, componibles y reutilizables. Cada uno se registra
// por id en defaultEffects() (effects_registry.go). Sirven tanto para el efecto
// principal de un status move (chance 100/0) como para un secundario de un move
// de daño (chance < 100): el mismo tipo, distinta data.

// effTarget indica a quién apunta un efecto de move.
type effTarget int

const (
	targetFoe  effTarget = iota // el defensor (status, secundarios)
	targetSelf                  // el atacante (Swords Dance, Calm Mind…)
)

// statusEffect inflige un status al defensor. chance es la probabilidad (100 =
// efecto principal como Will-O-Wisp; <100 = secundario como Flamethrower 10%).
type statusEffect struct {
	status StatusCondition
	chance int
}

func (s statusEffect) onHit(c *moveCtx) []Event {
	target := c.target()
	if target.Fainted {
		return nil
	}
	// La elegibilidad se chequea ANTES del roll: si el target es inmune por tipo
	// (o ya tiene status) no se consume RNG.
	if !c.e.canApplyStatus(c.state, c.targetSide, s.status) {
		return nil
	}
	if !rollChance(c.rng, s.chance) {
		return nil
	}
	return c.e.inflictStatus(c.state, c.targetSide, s.status, c.rng)
}

// boostEffect aplica stages de stat al atacante o al defensor.
type boostEffect struct {
	target effTarget
	boosts Boosts
	chance int // 0 = siempre (efecto principal)
}

func (b boostEffect) onHit(c *moveCtx) []Event {
	side := c.targetSide
	if b.target == targetSelf {
		side = c.userSide
	}
	if c.e.active(c.state, side).Fainted {
		return nil
	}
	if !rollChance(c.rng, b.chance) {
		return nil
	}
	return c.e.applyBoosts(c.state, side, b.boosts)
}

// weatherEffect setea el clima del campo (Sunny Day, Rain Dance, Sandstorm…).
type weatherEffect struct {
	weather string
	turns   int
}

func (w weatherEffect) onHit(c *moveCtx) []Event {
	if c.state.Field.Weather == w.weather {
		return nil // ya activo (las rocas que extienden a 8 quedan fuera de scope)
	}
	c.state.Field.Weather = w.weather
	c.state.Field.WeatherTurns = w.turns
	return []Event{{Kind: EventWeatherStarted, Reason: w.weather}}
}

// volatileEffect setea un volátil sobre el defensor (Leech Seed). Conoce la
// inmunidad propia del volátil (Leech Seed no afecta tipo Grass).
type volatileEffect struct {
	id string
}

func (v volatileEffect) onHit(c *moveCtx) []Event {
	target := c.target()
	if target.Fainted {
		return nil
	}
	if v.id == "leechseed" {
		for _, t := range c.e.typesOf(target) {
			if t == "grass" {
				return nil // Leech Seed falla contra Grass
			}
		}
	}
	if target.Volatiles == nil {
		target.Volatiles = map[string]any{}
	}
	if target.Volatiles[v.id] != nil {
		return nil
	}
	target.Volatiles[v.id] = true
	return []Event{{Kind: EventStatusInflicted, Side: c.targetSide, Slot: c.state.Sides[c.targetSide].Active, Status: v.id}}
}

// multiEffect ejecuta varios efectos en orden (un move que sube stats Y tiene un
// secundario, varios secundarios, etc.).
type multiEffect []moveEffect

func (m multiEffect) onHit(c *moveCtx) []Event {
	var evs []Event
	for _, eff := range m {
		evs = append(evs, eff.onHit(c)...)
	}
	return evs
}
