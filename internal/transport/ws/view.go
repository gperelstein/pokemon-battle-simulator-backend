package ws

import "github.com/gmperelstein/pokemon-battle-simulator-backend/internal/battle"

// publicView es el snapshot que ve un jugador concreto: su propio lado completo
// y el del rival con información limitada (sin moves/PP/item/ability ni HP
// exacto; solo porcentaje). Es deliberadamente simple para el MVP; afinar qué
// se revela (mons vistos, etc.) queda para más adelante.
type publicView struct {
	Turn     int        `json:"turn"`
	Phase    int        `json:"phase"`
	YourSide int        `json:"yourSide"`
	Weather  string     `json:"weather,omitempty"`
	Sides    [2]monSide `json:"sides"`
}

type monSide struct {
	Player string    `json:"player"`
	Active int       `json:"active"`
	Team   []monView `json:"team"`
}

type monView struct {
	Species   string `json:"species"`
	Level     int    `json:"level"`
	HPPercent int    `json:"hpPercent"`
	Status    string `json:"status,omitempty"`
	Fainted   bool   `json:"fainted"`
	Active    bool   `json:"active"`
	// Solo para el lado propio:
	HP      int        `json:"hp,omitempty"`
	MaxHP   int        `json:"maxHp,omitempty"`
	Item    string     `json:"item,omitempty"`
	Ability string     `json:"ability,omitempty"`
	Moves   []moveView `json:"moves,omitempty"`
}

type moveView struct {
	ID string `json:"id"`
	PP int    `json:"pp"`
}

// buildView arma la vista del estado para forSide.
func buildView(state *battle.State, forSide battle.SideID) publicView {
	v := publicView{
		Turn:     state.Turn,
		Phase:    int(state.Phase),
		YourSide: int(forSide),
		Weather:  state.Field.Weather,
	}
	for s := range state.Sides {
		own := battle.SideID(s) == forSide
		v.Sides[s] = buildSideView(&state.Sides[s], own)
	}
	return v
}

func buildSideView(side *battle.Side, own bool) monSide {
	ms := monSide{Player: side.Player.Name, Active: side.Active}
	for i := range side.Team {
		bp := &side.Team[i]
		if bp.Empty {
			continue
		}
		mv := monView{
			Species:   bp.Set.SpeciesID,
			Level:     bp.Set.Level,
			HPPercent: hpPercent(bp),
			Status:    string(bp.Status),
			Fainted:   bp.Fainted,
			Active:    i == side.Active,
		}
		if own {
			mv.HP = bp.HP
			mv.MaxHP = bp.MaxHP
			mv.Item = bp.Set.ItemID
			mv.Ability = bp.Set.AbilityID
			for slot, id := range bp.Set.Moves {
				if id != "" {
					mv.Moves = append(mv.Moves, moveView{ID: id, PP: bp.PP[slot]})
				}
			}
		}
		ms.Team = append(ms.Team, mv)
	}
	return ms
}

func hpPercent(bp *battle.BattlePokemon) int {
	if bp.MaxHP <= 0 {
		return 0
	}
	p := bp.HP * 100 / bp.MaxHP
	if p == 0 && bp.HP > 0 {
		return 1 // nunca mostrar 0% si todavía está vivo
	}
	return p
}
