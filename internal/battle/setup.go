package battle

import "github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"

// TeamSetup es la entrada de un lado para construir una batalla: el jugador y
// su equipo ya validado (ver pokemon.ValidateTeam).
type TeamSetup struct {
	Player PlayerInfo
	Team   pokemon.Team
}

// NewState construye el State inicial a partir de los dos equipos. Cablea los
// BattlePokemon (Set + slots vacíos) y deja el primer Pokémon no vacío de cada
// lado como activo, pero NO calcula stats ni emite eventos: eso lo hace
// Engine.Start (que tiene el Dex para resolver las base stats).
func NewState(genID int, seed uint64, format string, a, b TeamSetup) *State {
	st := &State{
		Gen:    genID,
		Seed:   seed,
		Format: format,
		Phase:  PhaseAwaitingActions,
	}
	setups := [2]TeamSetup{a, b}
	for i := range setups {
		s := &st.Sides[i]
		s.Player = setups[i].Player
		s.Conditions = map[string]int{}

		active := -1
		for j := 0; j < len(s.Team); j++ {
			set := setups[i].Team.Members[j]
			if set.SpeciesID == "" {
				s.Team[j].Empty = true
				continue
			}
			s.Team[j] = BattlePokemon{Set: set, Volatiles: map[string]any{}}
			if active == -1 {
				active = j
			}
		}
		if active == -1 {
			active = 0 // equipo vacío: caso degenerado, no debería pasar tras validar
		}
		s.Active = active
	}
	return st
}
