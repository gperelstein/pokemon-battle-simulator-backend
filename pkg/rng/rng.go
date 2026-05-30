package rng

import "math/rand/v2"

// RNG es una fuente pseudoaleatoria seedeable. Se usa en toda la batalla para
// que un mismo seed + secuencia de inputs reproduzca exactamente el mismo
// resultado (útil para tests, replays y debugging).
type RNG interface {
	// IntN devuelve un entero en [0, n). Pánico si n <= 0.
	IntN(n int) int
	// Float devuelve un float64 en [0, 1).
	Float() float64
	// Seed devuelve el seed original (para serializar y reproducir).
	Seed() uint64
}

// pcg implementa RNG sobre el generador PCG de math/rand/v2: determinístico,
// rápido y con buena calidad estadística. Guarda el seed original para poder
// serializarlo (el motor reconstruye el RNG de cada turno desde Seed+Turn).
type pcg struct {
	seed uint64
	r    *rand.Rand
}

// goldenGap mezcla el seed de 64 bits hacia el segundo word que necesita PCG
// (que toma dos uint64). Es la constante del número áureo en 64 bits, el mismo
// truco de mezcla que usa, p.ej., el hashing de Go, para que seeds consecutivos
// (0, 1, 2…) arranquen en estados bien separados.
const goldenGap = 0x9E3779B97F4A7C15

// New crea un RNG a partir de un seed. El mismo seed siempre produce la misma
// secuencia.
func New(seed uint64) RNG {
	src := rand.NewPCG(seed, seed^goldenGap)
	return &pcg{seed: seed, r: rand.New(src)}
}

func (g *pcg) IntN(n int) int { return g.r.IntN(n) }

func (g *pcg) Float() float64 { return g.r.Float64() }

func (g *pcg) Seed() uint64 { return g.seed }
