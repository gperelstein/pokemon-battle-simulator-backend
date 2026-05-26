package pokemon

// Type es un tipo elemental (Fire, Water, etc.). Se identifica por string id
// en minúsculas, igual que en el dataset de Showdown.
type Type string

// StatKey identifica una stat base/calculada.
type StatKey string

const (
	StatHP  StatKey = "hp"
	StatAtk StatKey = "atk"
	StatDef StatKey = "def"
	StatSpA StatKey = "spa"
	StatSpD StatKey = "spd"
	StatSpe StatKey = "spe"
)

// Stats es el vector de 6 stats. En Gen 1 SpA y SpD coinciden (Special único);
// se modela igual y el gen.Rules se encarga de la equivalencia.
type Stats struct {
	HP, Atk, Def, SpA, SpD, Spe int
}

// MoveCategory: físico, especial o estado.
type MoveCategory string

const (
	CategoryPhysical MoveCategory = "physical"
	CategorySpecial  MoveCategory = "special"
	CategoryStatus   MoveCategory = "status"
)

// Nature modifica stats finales (Gen 3+).
type Nature string
