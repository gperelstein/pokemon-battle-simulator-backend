package rng

// RNG es una fuente pseudoaleatoria seedeable. Se usa en toda la batalla para
// que un mismo seed + secuencia de inputs reproduzca exactamente el mismo
// resultado (útil para tests, replays y debugging).
type RNG interface {
	// IntN devuelve un entero en [0, n).
	IntN(n int) int
	// Float devuelve un float64 en [0, 1).
	Float() float64
	// Seed devuelve el seed original (para serializar y reproducir).
	Seed() uint64
}

// New crea un RNG a partir de un seed.
func New(seed uint64) RNG {
	panic("not implemented")
}
