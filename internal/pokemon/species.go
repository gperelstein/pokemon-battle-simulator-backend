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
	// EffectID referencia el efecto registrado en battle/effect. Un move sin
	// efecto registrado se trata como daño puro (o no-op si es status).
	EffectID string
	// Flags varias: contact, sound, punch, etc. Necesarias para interacciones.
	Flags map[string]bool
}

// Ability e Item se identifican por id; sus efectos viven en battle/effect.
type Ability struct {
	ID, Name string
}

type Item struct {
	ID, Name string
}
