package gen

import "fmt"

// Rules describe las mecánicas habilitadas y las fórmulas a usar para una
// generación concreta. La batalla recibe un Rules y todo cómputo dependiente
// de generación (daño, crítico, stats finales, etc.) pasa por él.
//
// La idea es que agregar una generación sea: definir las flags y las funciones
// que la diferencian, sin tocar el motor.
type Rules struct {
	Gen int

	// Mecánicas habilitadas
	HasAbilities     bool // Gen 3+
	HasItems         bool // Gen 2+
	HasNatures       bool // Gen 3+
	HasPhysSpecSplit bool // Gen 4+: cada move es fis/esp según su descripción, no según tipo
	HasFairyType     bool // Gen 6+
	HasTerrains      bool // Gen 6+
	HasZMoves        bool // Gen 7
	HasDynamax       bool // Gen 8
	HasTera          bool // Gen 9

	// Fórmulas inyectables (firmas a definir en internal/battle).
	// Se dejan como interface{} aquí para no atar acoplamiento circular;
	// los tipos concretos se definen en battle/.
	DamageFormula any
	CritFormula   any
	StatFormula   any
	TypeChart     any
}

// MinGen y MaxGen son las generaciones soportadas por el simulador.
const (
	MinGen = 1
	MaxGen = 9
)

// For devuelve las Rules de la generación indicada. Las mecánicas se derivan de
// umbrales por generación; las fórmulas (DamageFormula, etc.) se inyectan más
// adelante (paso 4 del plan) y por ahora quedan nil.
//
// Pánico si genID está fuera de [MinGen, MaxGen]: pasar una gen inválida es un
// error de programación, no una condición de runtime esperada.
func For(genID int) Rules {
	if genID < MinGen || genID > MaxGen {
		panic(fmt.Sprintf("gen.For: generación no soportada %d (rango %d..%d)", genID, MinGen, MaxGen))
	}
	return Rules{
		Gen:              genID,
		HasItems:         genID >= 2,
		HasAbilities:     genID >= 3,
		HasNatures:       genID >= 3,
		HasPhysSpecSplit: genID >= 4,
		HasFairyType:     genID >= 6,
		HasTerrains:      genID >= 6,
		HasZMoves:        genID == 7,
		HasDynamax:       genID == 8,
		HasTera:          genID == 9,
	}
}
