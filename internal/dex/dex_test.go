package dex_test

import (
	"testing"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

func load(t *testing.T) dex.Dex {
	t.Helper()
	d, err := dex.Load("testdata")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return d
}

func TestLoadMissingDir(t *testing.T) {
	if _, err := dex.Load("testdata/does-not-exist"); err == nil {
		t.Fatal("se esperaba error al cargar un directorio inexistente")
	}
}

func TestSpeciesLookup(t *testing.T) {
	d := load(t)

	sp, ok := d.Species(9, "charizard")
	if !ok {
		t.Fatal("charizard gen 9 no encontrado")
	}
	if sp.Name != "Charizard" {
		t.Errorf("Name = %q, want Charizard", sp.Name)
	}
	if len(sp.Types) != 2 || sp.Types[0] != "fire" || sp.Types[1] != "flying" {
		t.Errorf("Types = %v, want [fire flying]", sp.Types)
	}
	if sp.BaseStats.SpA != 109 {
		t.Errorf("gen9 SpA = %d, want 109", sp.BaseStats.SpA)
	}
	if len(sp.Abilities) != 2 || sp.Abilities[0] != "blaze" {
		t.Errorf("Abilities = %v, want [blaze solarpower]", sp.Abilities)
	}

	// Gen-specific: en gen 1 Charizard no tiene abilities y SpA=85 (Special único).
	sp1, ok := d.Species(1, "charizard")
	if !ok {
		t.Fatal("charizard gen 1 no encontrado")
	}
	if sp1.BaseStats.SpA != 85 {
		t.Errorf("gen1 SpA = %d, want 85", sp1.BaseStats.SpA)
	}
	if len(sp1.Abilities) != 0 {
		t.Errorf("gen1 Abilities = %v, want []", sp1.Abilities)
	}

	// Pikachu no existe en el fixture de gen 1.
	if _, ok := d.Species(1, "pikachu"); ok {
		t.Error("pikachu gen 1 no debería estar en el fixture")
	}
}

func TestMoveLookupGenSpecific(t *testing.T) {
	d := load(t)

	m9, ok := d.Move(9, "thunderbolt")
	if !ok || m9.Power != 90 {
		t.Errorf("gen9 thunderbolt power = %d (ok=%v), want 90", m9.Power, ok)
	}
	if m9.Category != pokemon.CategorySpecial || m9.Type != "electric" {
		t.Errorf("gen9 thunderbolt = %v/%v, want special/electric", m9.Category, m9.Type)
	}

	m1, ok := d.Move(1, "thunderbolt")
	if !ok || m1.Power != 95 {
		t.Errorf("gen1 thunderbolt power = %d (ok=%v), want 95", m1.Power, ok)
	}

	// accuracy 0 = nunca falla (Swift).
	sw, ok := d.Move(9, "swift")
	if !ok || sw.Accuracy != 0 {
		t.Errorf("swift accuracy = %d (ok=%v), want 0", sw.Accuracy, ok)
	}
}

func TestAbilityAndItemLookup(t *testing.T) {
	d := load(t)

	if _, ok := d.Ability(9, "blaze"); !ok {
		t.Error("blaze gen 9 no encontrado")
	}
	if _, ok := d.Ability(9, "nope"); ok {
		t.Error("ability inexistente devolvió ok")
	}
	if _, ok := d.Item(9, "leftovers"); !ok {
		t.Error("leftovers gen 9 no encontrado")
	}
	if _, ok := d.Item(9, "nope"); ok {
		t.Error("item inexistente devolvió ok")
	}
}

func TestTypeEffectiveness(t *testing.T) {
	d := load(t)
	tests := []struct {
		name     string
		attacker pokemon.Type
		defender []pokemon.Type
		want     float64
	}{
		{"super efectivo simple", "fire", []pokemon.Type{"grass"}, 2},
		{"resistido", "fire", []pokemon.Type{"water"}, 0.5},
		{"doble débil 4x", "electric", []pokemon.Type{"water", "flying"}, 4},
		{"inmune anula", "electric", []pokemon.Type{"ground"}, 0},
		{"neutral ausente = 1x", "fire", []pokemon.Type{"normal"}, 1},
		{"mixto resist*weak", "fire", []pokemon.Type{"water", "grass"}, 1},
		{"gen sin chart = 1x", "fire", []pokemon.Type{"grass"}, 2}, // gen 9 sí tiene
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.TypeEffectiveness(9, tc.attacker, tc.defender)
			if got != tc.want {
				t.Errorf("TypeEffectiveness(9, %s, %v) = %v, want %v", tc.attacker, tc.defender, got, tc.want)
			}
		})
	}

	// Generación sin typechart cargada devuelve 1x neutro.
	if got := d.TypeEffectiveness(3, "fire", []pokemon.Type{"grass"}); got != 1 {
		t.Errorf("gen sin chart = %v, want 1", got)
	}
}

func TestCanLearn(t *testing.T) {
	d := load(t)

	if !d.CanLearn(9, "charizard", "flamethrower") {
		t.Error("charizard debería aprender flamethrower en gen 9")
	}
	if d.CanLearn(9, "charizard", "thunderbolt") {
		t.Error("charizard NO debería aprender thunderbolt")
	}
	if d.CanLearn(9, "nope", "flamethrower") {
		t.Error("species inexistente no debería aprender nada")
	}
	if d.CanLearn(3, "charizard", "flamethrower") {
		t.Error("gen sin learnsets cargados no debería devolver true")
	}
}
