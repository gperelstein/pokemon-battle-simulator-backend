package battle

// Phase indica en qué punto del ciclo de turno está la batalla y, por lo tanto,
// qué input se espera. La transición entre fases la maneja el Engine.
type Phase int

const (
	// PhaseAwaitingActions: ambos jugadores deben enviar su Action (Move o
	// Switch voluntario) para que el turno pueda resolverse.
	PhaseAwaitingActions Phase = iota

	// PhaseResolving: el motor está ejecutando la cola de efectos. Esta fase
	// es interna; el cliente no manda inputs acá.
	PhaseResolving

	// PhaseAwaitingForcedSwitch: uno o ambos lados deben enviar un Switch
	// porque un efecto lo exige (faint, U-turn, Whirlwind, Baton Pass, etc.).
	// PendingSwitches indica qué lados deben responder.
	PhaseAwaitingForcedSwitch

	// PhaseEndOfTurn: efectos de fin de turno (clima, status, leftovers, etc.).
	// Interna.
	PhaseEndOfTurn

	// PhaseEnded: batalla terminada. State.Winner contiene el resultado.
	PhaseEnded
)
