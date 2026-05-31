package battle

import (
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
	"github.com/gperelstein/pokemon-battle-simulator-backend/pkg/rng"
)

func testEngine(t *testing.T, genID int) *Engine {
	t.Helper()
	d, err := dex.Load("../dex/testdata")
	if err != nil {
		t.Fatalf("dex.Load: %v", err)
	}
	return New(d, gen.For(genID))
}

func mon(species string, level int, stats pokemon.Stats) *BattlePokemon {
	return &BattlePokemon{Set: pokemon.Pokemon{SpeciesID: species, Level: level}, Stats: stats}
}

func TestEffectiveness(t *testing.T) {
	e := testEngine(t, 9)
	charizard := mon("charizard", 50, pokemon.Stats{})
	pikachu := mon("pikachu", 50, pokemon.Stats{})

	tests := []struct {
		move     pokemon.Move
		defender *BattlePokemon
		want     float64
	}{
		{pokemon.Move{Type: "electric"}, charizard, 2}, // electric → flying
		{pokemon.Move{Type: "fire"}, charizard, 0.5},   // fire → fire
		{pokemon.Move{Type: "ground"}, charizard, 0},   // ground → flying (inmune)
		{pokemon.Move{Type: "normal"}, pikachu, 1},     // sin entrada → neutro
	}
	for _, tc := range tests {
		if got := e.effectiveness(tc.move, tc.defender); got != tc.want {
			t.Errorf("effectiveness(%s vs %s) = %v, want %v", tc.move.Type, tc.defender.Set.SpeciesID, got, tc.want)
		}
	}
}

func TestHasSTAB(t *testing.T) {
	e := testEngine(t, 9)
	charizard := mon("charizard", 50, pokemon.Stats{})
	if !e.hasSTAB(charizard, pokemon.Move{Type: "fire"}) {
		t.Error("charizard debería tener STAB con fire")
	}
	if !e.hasSTAB(charizard, pokemon.Move{Type: "flying"}) {
		t.Error("charizard debería tener STAB con flying")
	}
	if e.hasSTAB(charizard, pokemon.Move{Type: "water"}) {
		t.Error("charizard NO debería tener STAB con water")
	}
}

func TestResolveCategoryPreSplit(t *testing.T) {
	// El move "electric" marcado como physical en los datos: con split (Gen 9)
	// respeta la marca; sin split (Gen 3) se decide por tipo (electric = special).
	mv := pokemon.Move{Type: "electric", Category: pokemon.CategoryPhysical}

	if got := testEngine(t, 9).resolveCategory(mv); got != pokemon.CategoryPhysical {
		t.Errorf("gen9 resolveCategory = %v, want physical (respeta el dato)", got)
	}
	if got := testEngine(t, 3).resolveCategory(mv); got != pokemon.CategorySpecial {
		t.Errorf("gen3 resolveCategory = %v, want special (electric es especial pre-split)", got)
	}
	// Un move de estado nunca cambia de categoría.
	status := pokemon.Move{Type: "normal", Category: pokemon.CategoryStatus}
	if got := testEngine(t, 3).resolveCategory(status); got != pokemon.CategoryStatus {
		t.Errorf("status resolveCategory = %v, want status", got)
	}
}

func TestApplyBoost(t *testing.T) {
	tests := []struct {
		stat, stage, want int
	}{
		{100, 0, 100},
		{100, 1, 150},
		{100, 2, 200},
		{100, 6, 400},
		{100, -1, 66},
		{100, -2, 50},
		{100, -6, 25},
		{100, 99, 400}, // clamp +6
		{100, -99, 25}, // clamp -6
	}
	for _, tc := range tests {
		if got := applyBoost(tc.stat, tc.stage); got != tc.want {
			t.Errorf("applyBoost(%d, %d) = %d, want %d", tc.stat, tc.stage, got, tc.want)
		}
	}
}

func TestAccuracyMultiplier(t *testing.T) {
	if got := accuracyMultiplier(0); got != 1 {
		t.Errorf("accuracyMultiplier(0) = %v, want 1", got)
	}
	if got := accuracyMultiplier(3); got != 2 { // (3+3)/3
		t.Errorf("accuracyMultiplier(3) = %v, want 2", got)
	}
	if got := accuracyMultiplier(-3); got != 0.5 { // 3/(3+3)
		t.Errorf("accuracyMultiplier(-3) = %v, want 0.5", got)
	}
}

// Con el mismo seed (mismo roll aleatorio) crit, STAB y efectividad deben subir
// el daño de forma comparable.
func TestCalcDamageModifiers(t *testing.T) {
	e := testEngine(t, 9)
	att := mon("charizard", 50, pokemon.Stats{Atk: 100, SpA: 120})
	def := mon("pikachu", 50, pokemon.Stats{Def: 80, SpD: 80})
	fire := pokemon.Move{Type: "fire", Category: pokemon.CategorySpecial, Power: 90}     // STAB para charizard
	normal := pokemon.Move{Type: "normal", Category: pokemon.CategorySpecial, Power: 90} // sin STAB

	base := e.calcDamage(att, def, normal, false, 1.0, rng.New(1))
	crit := e.calcDamage(att, def, normal, true, 1.0, rng.New(1))
	stab := e.calcDamage(att, def, fire, false, 1.0, rng.New(1))
	se := e.calcDamage(att, def, normal, false, 2.0, rng.New(1))

	if crit <= base {
		t.Errorf("crit (%d) debería superar a base (%d)", crit, base)
	}
	if stab <= base {
		t.Errorf("STAB (%d) debería superar a base (%d)", stab, base)
	}
	if se <= base {
		t.Errorf("super efectivo (%d) debería superar a base (%d)", se, base)
	}
	if base < 1 {
		t.Errorf("daño base = %d, want >= 1", base)
	}
}

func TestRollHitNeverMissAccuracyZero(t *testing.T) {
	e := testEngine(t, 9)
	att := mon("charizard", 50, pokemon.Stats{})
	def := mon("pikachu", 50, pokemon.Stats{})
	r := rng.New(1)
	move := pokemon.Move{Accuracy: 0} // 0 = nunca falla
	for i := 0; i < 100; i++ {
		if !e.rollHit(move, att, def, r) {
			t.Fatal("un move con accuracy 0 nunca debería fallar")
		}
	}
}

func TestRollHitImperfectAccuracy(t *testing.T) {
	e := testEngine(t, 9)
	att := mon("charizard", 50, pokemon.Stats{})
	def := mon("pikachu", 50, pokemon.Stats{})
	r := rng.New(1)
	move := pokemon.Move{Accuracy: 50} // 50%: sobre 200 tiradas hay aciertos y fallos
	misses, hits := 0, 0
	for i := 0; i < 200; i++ {
		if e.rollHit(move, att, def, r) {
			hits++
		} else {
			misses++
		}
	}
	if misses == 0 || hits == 0 {
		t.Errorf("con accuracy 50 se esperaban aciertos y fallos, got hits=%d misses=%d", hits, misses)
	}
}
