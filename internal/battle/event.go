package battle

// Event es un hecho atómico ocurrido durante la resolución de un turno. Los
// eventos se acumulan en una lista y se envían al cliente para que pueda
// animar la batalla en orden (similar al protocolo de Showdown |move|, |damage|,
// |-status|, etc.).
//
// Diseño: Event es una struct con discriminador Kind y campos opcionales.
// Se mantiene plano para serializar a JSON sin polimorfismo.
type Event struct {
	Kind EventKind

	Side   SideID
	Slot   int    // índice en el equipo, cuando aplica
	MoveID string
	Amount int    // daño, curación, etc.
	Stat   string // "atk", "spe", etc. para boosts
	Status string // status condition id
	Reason string // "miss", "immune", "crit", etc.
	Text   string // mensaje libre opcional
}

type EventKind int

const (
	EventMoveUsed EventKind = iota
	EventDamage
	EventHeal
	EventMiss
	EventCrit
	EventSuperEffective
	EventNotVeryEffective
	EventImmune
	EventStatusInflicted
	EventStatusCured
	EventBoostChanged
	EventFainted
	EventSwitchOut
	EventSwitchIn
	EventWeatherStarted
	EventWeatherEnded
	EventTerrainStarted
	EventTerrainEnded
	EventBattleEnded
	EventTurnStarted
	EventRequestForcedSwitch
)
