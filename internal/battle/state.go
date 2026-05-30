package battle

import (
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// State es el estado completo de una batalla en curso. Es el "documento" que
// el Engine recibe y devuelve transformado. Todo lo que necesita el motor para
// resolver un turno debe vivir acá (o ser puro — fórmulas, tablas).
//
// Diseño deliberado: State es serializable (todo son tipos planos), lo que
// permite snapshotting, replays y tests determinísticos.
type State struct {
	// Configuración inmutable
	Gen     int
	Seed    uint64 // semilla original del RNG; el RNG actual se reconstruye desde Seed+Turn
	Format  string

	// Lados (siempre 2 en singles)
	Sides [2]Side

	// Estado del campo: clima, terreno, gravity, trick room, etc.
	Field Field

	// Fase actual y qué se espera del usuario
	Phase           Phase
	PendingSwitches []SideID // lados que deben mandar ForcedSwitch en PhaseAwaitingForcedSwitch
	PendingActions  [2]*Action // acciones recibidas en PhaseAwaitingActions; nil si aún no llegó

	// Cola de efectos pendientes durante PhaseResolving (ver queue.go)
	Queue Queue

	// Contadores
	Turn int
	// ResumeCount cuenta las veces que la resolución se reanudó tras una pausa
	// (forced switch) dentro del mismo turno. Se usa para derivar un RNG
	// independiente por segmento de resolución (ver turnSeed), de modo que lo
	// que pasa después de un U-turn no quede correlacionado con lo de antes.
	// Se resetea a 0 al empezar cada turno.
	ResumeCount int

	// Resultado
	Winner *SideID // nil si la batalla sigue
}

// Side es el estado de un jugador: su equipo, quién está activo, hazards, etc.
type Side struct {
	Player  PlayerInfo
	Team    [6]BattlePokemon // posiciones vacías tienen Empty==true
	Active  int              // índice en Team del Pokémon activo
	Hazards Hazards          // Stealth Rock, Spikes, Toxic Spikes, Sticky Web
	// Side conditions: Reflect, Light Screen, Tailwind, etc.
	Conditions map[string]int // id → turnos restantes
}

type PlayerInfo struct {
	ID   string
	Name string
}

// BattlePokemon es la instancia mutable durante la batalla. Refiere a la
// Pokemon original (Set) y agrega todo el estado volátil.
type BattlePokemon struct {
	Empty bool // slot vacío del equipo

	Set pokemon.Pokemon // datos del set elegido (referencia por valor para que State sea autocontenido)

	// Stats finales calculadas al cargar el Pokémon (según gen.Rules.StatFormula).
	MaxHP int
	HP    int
	Stats pokemon.Stats

	// Estado en batalla
	Status     StatusCondition // burn, poison, sleep, etc.
	StatusData StatusData      // contadores (sleep turns restantes, toxic counter)
	Boosts     Boosts          // -6..+6 por stat
	Volatiles  map[string]any  // confusion, leech seed, taunt, etc. — datos arbitrarios por id

	// PP actual por slot de move (paralelo a Set.Moves)
	PP [4]int

	// Flags de turno (se resetean al cambiar o terminar turno según corresponda)
	LastMoveUsed string
	Fainted      bool
}

// Boosts es el vector -6..+6 por stat. Accuracy/Evasion también van acá.
type Boosts struct {
	Atk, Def, SpA, SpD, Spe, Acc, Eva int
}

type StatusCondition string

const (
	StatusNone     StatusCondition = ""
	StatusBurn     StatusCondition = "brn"
	StatusFreeze   StatusCondition = "frz"
	StatusParalyze StatusCondition = "par"
	StatusPoison   StatusCondition = "psn"
	StatusToxic    StatusCondition = "tox"
	StatusSleep    StatusCondition = "slp"
)

type StatusData struct {
	SleepTurns int
	ToxicCount int
}

// Field engloba el estado compartido por ambos lados.
type Field struct {
	Weather     string // "sun", "rain", "sand", "hail", "snow", ""
	WeatherTurns int
	Terrain     string // "electric", "grassy", "misty", "psychic", ""
	TerrainTurns int
	TrickRoom    int // turnos restantes; 0 = inactivo
	Gravity      int
}

type Hazards struct {
	StealthRock bool
	Spikes      int // 0..3 capas
	ToxicSpikes int // 0..2
	StickyWeb   bool
}
