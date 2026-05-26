package battle

// SideID identifica al jugador. En singles solo hay dos lados.
type SideID int

const (
	SideA SideID = 0
	SideB SideID = 1
)

// ActionKind discrimina el tipo de acción del jugador.
type ActionKind int

const (
	ActionMove         ActionKind = iota // usar un movimiento del Pokémon activo
	ActionSwitch                         // cambio voluntario al inicio del turno
	ActionForcedSwitch                   // cambio forzado (faint, U-turn, etc.)
	ActionForfeit                        // rendirse
)

// Action es lo que un jugador envía cada vez que el motor le pide input.
// Solo uno de Move/Switch está poblado según Kind.
type Action struct {
	Kind   ActionKind
	Side   SideID
	Move   *MoveAction
	Switch *SwitchAction
}

// MoveAction: el jugador elige uno de los 4 moves del Pokémon activo.
// MoveSlot es 0..3.
type MoveAction struct {
	MoveSlot int
	// Tera, Mega, Dynamax, Z-move se modelarán acá cuando lleguen las gens
	// correspondientes (flags booleanas o subcampos).
}

// SwitchAction: el jugador elige a qué slot del equipo cambia. TeamSlot es
// 0..5; debe apuntar a un Pokémon vivo distinto del activo.
type SwitchAction struct {
	TeamSlot int
}
