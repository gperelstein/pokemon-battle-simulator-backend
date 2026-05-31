package pokemon_test

import (
	"strings"
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// fakeDex es un Dex en memoria (una sola "generación") para los tests del
// validador; ignora el parámetro gen.
type fakeDex struct {
	species   map[string]pokemon.Species
	moves     map[string]pokemon.Move
	abilities map[string]pokemon.Ability
	items     map[string]pokemon.Item
	learn     map[string]map[string]bool
}

func (f *fakeDex) Species(_ int, id string) (pokemon.Species, bool) {
	s, ok := f.species[id]
	return s, ok
}
func (f *fakeDex) Move(_ int, id string) (pokemon.Move, bool) {
	m, ok := f.moves[id]
	return m, ok
}
func (f *fakeDex) Ability(_ int, id string) (pokemon.Ability, bool) {
	a, ok := f.abilities[id]
	return a, ok
}
func (f *fakeDex) Item(_ int, id string) (pokemon.Item, bool) {
	it, ok := f.items[id]
	return it, ok
}
func (f *fakeDex) CanLearn(_ int, speciesID, moveID string) bool {
	return f.learn[speciesID][moveID]
}

func newDex() *fakeDex {
	return &fakeDex{
		species: map[string]pokemon.Species{
			"charizard": {ID: "charizard", Name: "Charizard", Types: []pokemon.Type{"fire", "flying"}, Abilities: []string{"blaze", "solarpower"}},
		},
		moves: map[string]pokemon.Move{
			"flamethrower": {ID: "flamethrower", Name: "Flamethrower"},
			"airslash":     {ID: "airslash", Name: "Air Slash"},
			"thunderbolt":  {ID: "thunderbolt", Name: "Thunderbolt"},
		},
		abilities: map[string]pokemon.Ability{
			"blaze":      {ID: "blaze", Name: "Blaze"},
			"solarpower": {ID: "solarpower", Name: "Solar Power"},
			"static":     {ID: "static", Name: "Static"},
		},
		items: map[string]pokemon.Item{
			"leftovers": {ID: "leftovers", Name: "Leftovers"},
		},
		learn: map[string]map[string]bool{
			"charizard": {"flamethrower": true, "airslash": true},
		},
	}
}

// validCharizard devuelve un set legal de gen 9.
func validCharizard() pokemon.Pokemon {
	return pokemon.Pokemon{
		SpeciesID: "charizard",
		Level:     50,
		Nature:    "timid",
		IVs:       pokemon.Stats{HP: 31, Atk: 31, Def: 31, SpA: 31, SpD: 31, Spe: 31},
		EVs:       pokemon.Stats{SpA: 252, Spe: 252, HP: 4},
		AbilityID: "blaze",
		ItemID:    "leftovers",
		Moves:     [4]string{"flamethrower", "airslash", "", ""},
	}
}

func teamOf(p pokemon.Pokemon) pokemon.Team {
	var t pokemon.Team
	t.Members[0] = p
	return t
}

func TestValidateTeam_Valid(t *testing.T) {
	if err := pokemon.ValidateTeam(teamOf(validCharizard()), gen.For(9), newDex()); err != nil {
		t.Fatalf("equipo válido devolvió error: %v", err)
	}
}

func TestValidateTeam_Empty(t *testing.T) {
	err := pokemon.ValidateTeam(pokemon.Team{}, gen.For(9), newDex())
	if err == nil || !strings.Contains(err.Error(), "ningún Pokémon") {
		t.Fatalf("equipo vacío: err = %v", err)
	}
}

// mutate aplica f a un set válido y valida en gen 9, esperando que el error
// contenga want.
func expectErr(t *testing.T, want string, f func(*pokemon.Pokemon)) {
	t.Helper()
	p := validCharizard()
	f(&p)
	err := pokemon.ValidateTeam(teamOf(p), gen.For(9), newDex())
	if err == nil {
		t.Fatalf("se esperaba error que contenga %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want que contenga %q", err.Error(), want)
	}
}

func TestValidateTeam_Errors(t *testing.T) {
	t.Run("species inexistente", func(t *testing.T) {
		expectErr(t, "species inexistente", func(p *pokemon.Pokemon) { p.SpeciesID = "missingno" })
	})
	t.Run("nivel fuera de rango", func(t *testing.T) {
		expectErr(t, "nivel", func(p *pokemon.Pokemon) { p.Level = 101 })
	})
	t.Run("IV fuera de rango", func(t *testing.T) {
		expectErr(t, "IV", func(p *pokemon.Pokemon) { p.IVs.Spe = 32 })
	})
	t.Run("EV individual fuera de rango", func(t *testing.T) {
		expectErr(t, "EV", func(p *pokemon.Pokemon) { p.EVs.HP = 256 })
	})
	t.Run("suma de EVs excede", func(t *testing.T) {
		expectErr(t, "suma de EVs", func(p *pokemon.Pokemon) {
			p.EVs = pokemon.Stats{HP: 252, Atk: 252, Def: 252}
		})
	})
	t.Run("naturaleza inválida", func(t *testing.T) {
		expectErr(t, "naturaleza", func(p *pokemon.Pokemon) { p.Nature = "supersaiyan" })
	})
	t.Run("ability ilegal para la especie", func(t *testing.T) {
		expectErr(t, "no es legal", func(p *pokemon.Pokemon) { p.AbilityID = "static" })
	})
	t.Run("ability inexistente", func(t *testing.T) {
		expectErr(t, "ability inexistente", func(p *pokemon.Pokemon) { p.AbilityID = "nope" })
	})
	t.Run("item inexistente", func(t *testing.T) {
		expectErr(t, "item inexistente", func(p *pokemon.Pokemon) { p.ItemID = "nope" })
	})
	t.Run("move inexistente", func(t *testing.T) {
		expectErr(t, "move inexistente", func(p *pokemon.Pokemon) { p.Moves[2] = "nope" })
	})
	t.Run("move no aprendible", func(t *testing.T) {
		expectErr(t, "no puede aprender", func(p *pokemon.Pokemon) { p.Moves[2] = "thunderbolt" })
	})
	t.Run("move repetido", func(t *testing.T) {
		expectErr(t, "move repetido", func(p *pokemon.Pokemon) { p.Moves[1] = "flamethrower" })
	})
	t.Run("sin moves", func(t *testing.T) {
		expectErr(t, "ningún move", func(p *pokemon.Pokemon) { p.Moves = [4]string{} })
	})
}

// En gen 1 no hay naturalezas/abilities/items y los IVs son DVs (0..15).
func TestValidateTeam_Gen1Mechanics(t *testing.T) {
	d := newDex()

	// Set "gen1": nature/ability/item se ignoran; IVs <= 15.
	p := pokemon.Pokemon{
		SpeciesID: "charizard",
		Level:     50,
		Nature:    "supersaiyan", // se ignora en gen 1
		AbilityID: "static",      // se ignora en gen 1
		ItemID:    "nope",        // se ignora en gen 1
		IVs:       pokemon.Stats{HP: 15, Atk: 15, Def: 15, SpA: 15, SpD: 15, Spe: 15},
		Moves:     [4]string{"flamethrower", "", "", ""},
	}
	if err := pokemon.ValidateTeam(teamOf(p), gen.For(1), d); err != nil {
		t.Fatalf("set gen1 válido devolvió error: %v", err)
	}

	// IV 31 es ilegal en gen 1 (máximo DV = 15).
	p.IVs.Spe = 31
	err := pokemon.ValidateTeam(teamOf(p), gen.For(1), d)
	if err == nil || !strings.Contains(err.Error(), "0..15") {
		t.Fatalf("IV 31 en gen 1: err = %v", err)
	}
}

func TestFor_PanicsOnInvalidGen(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("gen.For(0) debería panic")
		}
	}()
	gen.For(0)
}
