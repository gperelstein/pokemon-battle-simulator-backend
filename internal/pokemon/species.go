package pokemon

// Species son los datos estáticos del Pokémon. Provienen del dex y no cambian
// durante la batalla.
type Species struct {
	ID        string // "charizard"
	Name      string // "Charizard"
	Types     []Type
	BaseStats Stats
	Abilities []string // ids de abilities posibles
	// Variaciones por generación (ej.: tipo en Gen 1 vs 2) se resuelven en el
	// loader del dex devolviendo la Species correcta según gen.
}

// Move estático: lo que define al movimiento, no su ejecución concreta.
type Move struct {
	ID       string
	Name     string
	Type     Type
	Category MoveCategory
	Power    int    // 0 si no aplica
	Accuracy int    // 0..100; 0 = nunca falla
	PP       int    // máximo
	Priority int    // típicamente -7..+5
	Target   string // "normal", "self", "allAdjacentFoes", etc.
	// EffectID: heredado del diseño original. El sistema de efectos (paso 8)
	// keyea por el id del propio move (convención Showdown), así que este campo
	// quedó sin uso; se mantiene por compatibilidad y es candidato a borrarse.
	EffectID string
	// SelfSwitch != "" hace que el atacante cambie tras usar el move (U-turn,
	// Volt Switch, Teleport…). Valores: "true", "copyvolatile" (Baton Pass,
	// copia boosts/volátiles), "shedtail". El cambio lo elige el jugador.
	SelfSwitch string `json:"selfSwitch"`
	// ForceSwitch saca al defensor a un Pokémon aleatorio del banco (Roar,
	// Whirlwind, Dragon Tail, Circle Throw).
	ForceSwitch bool `json:"forceSwitch"`
	// Flags varias: contact, sound, punch, etc. Necesarias para interacciones.
	Flags map[string]bool
}

// Ability e Item se identifican por id; sus efectos viven en internal/battle
// (effects*.go), registrados por ese mismo id.
type Ability struct {
	ID, Name string
}

type Item struct {
	ID, Name string
}
