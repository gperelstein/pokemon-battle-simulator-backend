package dex

import "github.com/gmperelstein/pokemon-battle-simulator-backend/internal/pokemon"

// Dex es el índice de datos estáticos del juego: species, moves, abilities,
// items, type chart. Se carga una vez al arrancar el servidor desde JSON
// exportado del dataset de Pokémon Showdown.
//
// Los lookups son por id (lowercase, sin caracteres especiales — convención
// Showdown) y por generación: el mismo id puede tener datos distintos por
// gen (ej.: Sableye gana Prankster en Gen 5).
type Dex interface {
	Species(gen int, id string) (pokemon.Species, bool)
	Move(gen int, id string) (pokemon.Move, bool)
	Ability(gen int, id string) (pokemon.Ability, bool)
	Item(gen int, id string) (pokemon.Item, bool)

	// TypeEffectiveness devuelve el multiplicador (0, 0.25, 0.5, 1, 2, 4)
	// de attackType contra defenderTypes para la generación dada.
	TypeEffectiveness(gen int, attackType pokemon.Type, defenderTypes []pokemon.Type) float64
}

// Load construye un Dex leyendo los JSON del directorio dataPath.
// Esperado: dataPath/{species,moves,abilities,items,typechart}.json
func Load(dataPath string) (Dex, error) {
	panic("not implemented")
}
