package pokemon

import (
	"errors"
	"fmt"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/gen"
)

// Dex es el subconjunto de dex.Dex que necesita la validación de equipos.
// Se declara acá (en vez de importar internal/dex) para evitar un ciclo de
// imports: dex importa pokemon, así que pokemon no puede importar dex. El
// *dex.data concreto satisface esta interfaz estructuralmente.
type Dex interface {
	Species(gen int, id string) (Species, bool)
	Move(gen int, id string) (Move, bool)
	Ability(gen int, id string) (Ability, bool)
	Item(gen int, id string) (Item, bool)
	CanLearn(gen int, speciesID, moveID string) bool
}

// Límites de configuración de un set.
const (
	minLevel    = 1
	maxLevel    = 100
	maxIV       = 31    // Gen 3+
	maxDV       = 15    // Gen 1/2 (DVs)
	maxEV       = 255   // Gen 3+
	maxEVTotal  = 510   // Gen 3+
	maxStatExp  = 65535 // Gen 1/2 (stat experience)
	maxMoveSlot = 4
)

// ValidateTeam verifica que un equipo sea legal para las reglas de la
// generación dada, consultando el dex para resolver species/moves/abilities/
// items y la legalidad de aprendizaje. Devuelve un error que agrega todos los
// problemas encontrados (errors.Join), o nil si el equipo es válido.
//
// Cubre: al menos un Pokémon; species existente; nivel en rango; IVs/EVs en
// rango y suma legal según gen; naturaleza válida (si la gen las usa); ability
// válida y aprendible por la especie (si la gen las usa); item existente (si la
// gen los usa); 1..4 moves sin repetir, existentes y aprendibles por la especie.
func ValidateTeam(t Team, rules gen.Rules, d Dex) error {
	var errs []error

	hasMember := false
	for i := range t.Members {
		m := t.Members[i]
		if m.SpeciesID == "" {
			continue // slot vacío
		}
		hasMember = true
		validateMember(&errs, i, m, rules, d)
	}

	if !hasMember {
		errs = append(errs, errors.New("el equipo no tiene ningún Pokémon"))
	}

	return errors.Join(errs...)
}

func validateMember(errs *[]error, slot int, m Pokemon, rules gen.Rules, d Dex) {
	// Prefijo para ubicar el error: usa apodo o species.
	label := m.SpeciesID
	if m.Nickname != "" {
		label = fmt.Sprintf("%s (%s)", m.Nickname, m.SpeciesID)
	}
	fail := func(format string, args ...any) {
		*errs = append(*errs, fmt.Errorf("slot %d %s: %s", slot+1, label, fmt.Sprintf(format, args...)))
	}

	species, ok := d.Species(rules.Gen, m.SpeciesID)
	if !ok {
		fail("species inexistente en gen %d", rules.Gen)
		return // sin species no se puede validar el resto
	}

	if m.Level < minLevel || m.Level > maxLevel {
		fail("nivel %d fuera de rango (%d..%d)", m.Level, minLevel, maxLevel)
	}

	validateIVs(fail, m.IVs, rules)
	validateEVs(fail, m.EVs, rules)

	if rules.HasNatures && m.Nature != "" && !validNatures[m.Nature] {
		fail("naturaleza inválida %q", m.Nature)
	}

	validateAbility(fail, m, species, rules, d)

	if rules.HasItems && m.ItemID != "" {
		if _, ok := d.Item(rules.Gen, m.ItemID); !ok {
			fail("item inexistente %q en gen %d", m.ItemID, rules.Gen)
		}
	}

	validateMoves(fail, m, rules, d)
}

func validateIVs(fail func(string, ...any), ivs Stats, rules gen.Rules) {
	max := maxIV
	if rules.Gen <= 2 {
		max = maxDV
	}
	for _, s := range statList(ivs) {
		if s.val < 0 || s.val > max {
			fail("IV %s=%d fuera de rango (0..%d)", s.name, s.val, max)
		}
	}
}

func validateEVs(fail func(string, ...any), evs Stats, rules gen.Rules) {
	if rules.Gen <= 2 {
		// Stat experience: 0..65535 por stat, sin tope de suma.
		for _, s := range statList(evs) {
			if s.val < 0 || s.val > maxStatExp {
				fail("stat exp %s=%d fuera de rango (0..%d)", s.name, s.val, maxStatExp)
			}
		}
		return
	}
	total := 0
	for _, s := range statList(evs) {
		if s.val < 0 || s.val > maxEV {
			fail("EV %s=%d fuera de rango (0..%d)", s.name, s.val, maxEV)
		}
		total += s.val
	}
	if total > maxEVTotal {
		fail("suma de EVs %d excede el máximo (%d)", total, maxEVTotal)
	}
}

func validateAbility(fail func(string, ...any), m Pokemon, species Species, rules gen.Rules, d Dex) {
	if !rules.HasAbilities || m.AbilityID == "" {
		return // gen sin abilities, o ability por defecto: nada que validar
	}
	if _, ok := d.Ability(rules.Gen, m.AbilityID); !ok {
		fail("ability inexistente %q en gen %d", m.AbilityID, rules.Gen)
		return
	}
	for _, a := range species.Abilities {
		if a == m.AbilityID {
			return
		}
	}
	fail("ability %q no es legal para %s", m.AbilityID, species.ID)
}

func validateMoves(fail func(string, ...any), m Pokemon, rules gen.Rules, d Dex) {
	seen := make(map[string]bool, maxMoveSlot)
	count := 0
	for _, moveID := range m.Moves {
		if moveID == "" {
			continue
		}
		count++
		if seen[moveID] {
			fail("move repetido %q", moveID)
			continue
		}
		seen[moveID] = true

		if _, ok := d.Move(rules.Gen, moveID); !ok {
			fail("move inexistente %q en gen %d", moveID, rules.Gen)
			continue
		}
		if !d.CanLearn(rules.Gen, m.SpeciesID, moveID) {
			fail("%s no puede aprender %q en gen %d", m.SpeciesID, moveID, rules.Gen)
		}
	}
	if count == 0 {
		fail("no tiene ningún move")
	}
}

type namedStat struct {
	name string
	val  int
}

func statList(s Stats) [6]namedStat {
	return [6]namedStat{
		{"hp", s.HP}, {"atk", s.Atk}, {"def", s.Def},
		{"spa", s.SpA}, {"spd", s.SpD}, {"spe", s.Spe},
	}
}

// validNatures son las 25 naturalezas (ids Showdown, lowercase).
var validNatures = map[Nature]bool{
	"adamant": true, "bashful": true, "bold": true, "brave": true, "calm": true,
	"careful": true, "docile": true, "gentle": true, "hardy": true, "hasty": true,
	"impish": true, "jolly": true, "lax": true, "lonely": true, "mild": true,
	"modest": true, "naive": true, "naughty": true, "quiet": true, "quirky": true,
	"rash": true, "relaxed": true, "sassy": true, "serious": true, "timid": true,
}
