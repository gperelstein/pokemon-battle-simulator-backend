package gen

// Rules describe las mecánicas habilitadas y las fórmulas a usar para una
// generación concreta. La batalla recibe un Rules y todo cómputo dependiente
// de generación (daño, crítico, stats finales, etc.) pasa por él.
//
// La idea es que agregar una generación sea: definir las flags y las funciones
// que la diferencian, sin tocar el motor.
type Rules struct {
	Gen int

	// Mecánicas habilitadas
	HasAbilities      bool // Gen 3+
	HasItems          bool // Gen 2+
	HasNatures        bool // Gen 3+
	HasPhysSpecSplit  bool // Gen 4+: cada move es fis/esp según su descripción, no según tipo
	HasFairyType      bool // Gen 6+
	HasTerrains       bool // Gen 6+
	HasZMoves         bool // Gen 7
	HasDynamax        bool // Gen 8
	HasTera           bool // Gen 9

	// Fórmulas inyectables (firmas a definir en battle/effect).
	// Se dejan como interface{} aquí para no atar acoplamiento circular;
	// los tipos concretos se definen en battle/.
	DamageFormula any
	CritFormula   any
	StatFormula   any
	TypeChart     any
}

// For devuelve las Rules de la generación indicada.
// Implementación pendiente: tabla por gen.
func For(genID int) Rules {
	panic("not implemented")
}
