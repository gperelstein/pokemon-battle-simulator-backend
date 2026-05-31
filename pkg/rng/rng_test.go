package rng_test

import (
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/pkg/rng"
)

// draw saca una secuencia mezclada de IntN/Float para comparar reproducibilidad.
func draw(r rng.RNG, n int) []float64 {
	out := make([]float64, 0, n*2)
	for i := 0; i < n; i++ {
		out = append(out, float64(r.IntN(100)))
		out = append(out, r.Float())
	}
	return out
}

func equal(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSeedReproducible(t *testing.T) {
	a := draw(rng.New(42), 50)
	b := draw(rng.New(42), 50)
	if !equal(a, b) {
		t.Fatal("mismo seed produjo secuencias distintas")
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a := draw(rng.New(1), 50)
	b := draw(rng.New(2), 50)
	if equal(a, b) {
		t.Fatal("seeds distintos produjeron la misma secuencia")
	}
}

func TestSeedAccessor(t *testing.T) {
	const s = uint64(0xDEADBEEF)
	if got := rng.New(s).Seed(); got != s {
		t.Fatalf("Seed() = %#x, want %#x", got, s)
	}
}

func TestIntNRange(t *testing.T) {
	r := rng.New(7)
	for _, n := range []int{1, 2, 5, 16, 100} {
		for i := 0; i < 1000; i++ {
			v := r.IntN(n)
			if v < 0 || v >= n {
				t.Fatalf("IntN(%d) = %d fuera de [0,%d)", n, v, n)
			}
		}
	}
}

func TestIntN1AlwaysZero(t *testing.T) {
	r := rng.New(123)
	for i := 0; i < 100; i++ {
		if v := r.IntN(1); v != 0 {
			t.Fatalf("IntN(1) = %d, want 0", v)
		}
	}
}

func TestFloatRange(t *testing.T) {
	r := rng.New(99)
	for i := 0; i < 10000; i++ {
		f := r.Float()
		if f < 0 || f >= 1 {
			t.Fatalf("Float() = %v fuera de [0,1)", f)
		}
	}
}

func TestIntNPanicsOnNonPositive(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("IntN(0) debería panic")
		}
	}()
	rng.New(1).IntN(0)
}

// Las secuencias de seeds consecutivos no deben quedar correlacionadas (sanity
// check del mezclado del segundo word de PCG).
func TestConsecutiveSeedsDecorrelated(t *testing.T) {
	a := draw(rng.New(0), 20)
	b := draw(rng.New(1), 20)
	if equal(a, b) {
		t.Fatal("seeds 0 y 1 produjeron la misma secuencia")
	}
}
