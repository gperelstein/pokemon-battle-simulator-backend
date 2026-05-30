package dex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// Dex es el índice de datos estáticos del juego: species, moves, abilities,
// items, type chart y learnsets. Se carga una vez al arrancar el servidor desde
// JSON exportado del dataset de Pokémon Showdown (ver scripts/showdown-export).
//
// Los lookups son por id (lowercase, sin caracteres especiales — convención
// Showdown) y por generación: el mismo id puede tener datos distintos por
// gen (ej.: Thunderbolt tiene 95 de poder en Gen 1 y 90 en Gen 4+; Charizard
// no tiene ability en Gen 1).
type Dex interface {
	Species(gen int, id string) (pokemon.Species, bool)
	Move(gen int, id string) (pokemon.Move, bool)
	Ability(gen int, id string) (pokemon.Ability, bool)
	Item(gen int, id string) (pokemon.Item, bool)

	// TypeEffectiveness devuelve el multiplicador (0, 0.25, 0.5, 1, 2, 4)
	// de attackType contra defenderTypes para la generación dada.
	TypeEffectiveness(gen int, attackType pokemon.Type, defenderTypes []pokemon.Type) float64

	// CanLearn indica si speciesID puede aprender moveID en la generación dada.
	// Membership aplanado (especie + cadena prevo); no modela legalidad fina
	// (egg moves, eventos, restricciones de fuente). Suficiente para validar
	// equipos en el MVP.
	CanLearn(gen int, speciesID, moveID string) bool
}

// data es la implementación concreta del Dex: mapas indexados por gen → id.
type data struct {
	species   map[int]map[string]pokemon.Species
	moves     map[int]map[string]pokemon.Move
	abilities map[int]map[string]pokemon.Ability
	items     map[int]map[string]pokemon.Item
	// typechart[gen][attacker][defender] = multiplicador. Solo se guardan los
	// valores distintos de 1x; un lookup ausente significa 1x.
	typechart map[int]map[string]map[string]float64
	// learnsets[gen][speciesID][moveID] = true.
	learnsets map[int]map[string]map[string]bool
}

// Load construye un Dex leyendo los JSON del directorio dataPath.
// Espera: dataPath/{species,moves,abilities,items,typechart,learnsets}.json,
// cada uno un objeto top-level indexado por generación (string) → id → dato.
func Load(dataPath string) (Dex, error) {
	d := &data{}

	if err := readJSON(dataPath, "species.json", &d.species); err != nil {
		return nil, err
	}
	if err := readJSON(dataPath, "moves.json", &d.moves); err != nil {
		return nil, err
	}
	if err := readJSON(dataPath, "abilities.json", &d.abilities); err != nil {
		return nil, err
	}
	if err := readJSON(dataPath, "items.json", &d.items); err != nil {
		return nil, err
	}
	if err := readJSON(dataPath, "typechart.json", &d.typechart); err != nil {
		return nil, err
	}

	// learnsets se exportan como listas; las convertimos a sets para CanLearn O(1).
	var rawLearnsets map[int]map[string][]string
	if err := readJSON(dataPath, "learnsets.json", &rawLearnsets); err != nil {
		return nil, err
	}
	d.learnsets = make(map[int]map[string]map[string]bool, len(rawLearnsets))
	for gen, bySpecies := range rawLearnsets {
		genSet := make(map[string]map[string]bool, len(bySpecies))
		for speciesID, moves := range bySpecies {
			set := make(map[string]bool, len(moves))
			for _, moveID := range moves {
				set[moveID] = true
			}
			genSet[speciesID] = set
		}
		d.learnsets[gen] = genSet
	}

	return d, nil
}

func readJSON(dir, name string, dst any) error {
	path := filepath.Join(dir, name)
	bytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("dex: leyendo %s: %w", name, err)
	}
	if err := json.Unmarshal(bytes, dst); err != nil {
		return fmt.Errorf("dex: parseando %s: %w", name, err)
	}
	return nil
}

func (d *data) Species(gen int, id string) (pokemon.Species, bool) {
	s, ok := d.species[gen][id]
	return s, ok
}

func (d *data) Move(gen int, id string) (pokemon.Move, bool) {
	m, ok := d.moves[gen][id]
	return m, ok
}

func (d *data) Ability(gen int, id string) (pokemon.Ability, bool) {
	a, ok := d.abilities[gen][id]
	return a, ok
}

func (d *data) Item(gen int, id string) (pokemon.Item, bool) {
	it, ok := d.items[gen][id]
	return it, ok
}

func (d *data) TypeEffectiveness(gen int, attackType pokemon.Type, defenderTypes []pokemon.Type) float64 {
	row, ok := d.typechart[gen][string(attackType)]
	if !ok {
		return 1
	}
	mult := 1.0
	for _, dt := range defenderTypes {
		if m, found := row[string(dt)]; found {
			mult *= m
		}
	}
	return mult
}

func (d *data) CanLearn(gen int, speciesID, moveID string) bool {
	return d.learnsets[gen][speciesID][moveID]
}
