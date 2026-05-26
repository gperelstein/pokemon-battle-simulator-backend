package battle

// Queue es la cola de efectos pendientes que el motor procesa durante
// PhaseResolving. Cada elemento es un paso atómico: ejecutar un switch,
// ejecutar un move, aplicar end-of-turn de un Pokémon, etc.
//
// El procesamiento es iterativo: se hace pop del primer elemento, se ejecuta,
// y la ejecución puede empujar nuevos elementos al frente (por ej., un move
// que causa faint empuja un "request forced switch") o al final (efectos
// secundarios diferidos). Esto permite modelar interacciones complejas sin
// recursión y manteniendo el orden bien definido.
//
// Si la ejecución de un elemento requiere input del usuario (forced switch),
// el motor sale de PhaseResolving a PhaseAwaitingForcedSwitch dejando el
// resto de la cola intacta. Cuando llega el input, se vuelve a PhaseResolving
// y se continúa.
type Queue struct {
	Items []QueueItem
}

// QueueItem es un paso pendiente. Kind discrimina la unión.
type QueueItem struct {
	Kind QueueItemKind

	// Campos según Kind:
	Side     SideID // dueño del efecto, cuando aplica
	MoveSlot int    // para QueueMove
	Switch   *SwitchAction // para QueueSwitch (incluye forced)
	EffectID string // para QueueCustom (ability/item/move custom)
}

type QueueItemKind int

const (
	QueueSwitch        QueueItemKind = iota // ejecutar un switch (voluntario o forzado)
	QueueMove                                // ejecutar el move elegido por Side
	QueueResidualSide                        // residuales por lado (status damage, leech seed, leftovers...)
	QueueResidualField                       // residuales del campo (clima, terreno)
	QueueRequestForcedSwitch                 // marca: el motor debe pausar y pedir switch
	QueueCustom                              // efecto referenciado por id (ability/item)
)

func (q *Queue) PushBack(it QueueItem)  { q.Items = append(q.Items, it) }
func (q *Queue) PushFront(it QueueItem) { q.Items = append([]QueueItem{it}, q.Items...) }
func (q *Queue) Pop() (QueueItem, bool) {
	if len(q.Items) == 0 {
		return QueueItem{}, false
	}
	it := q.Items[0]
	q.Items = q.Items[1:]
	return it, true
}
func (q *Queue) Empty() bool { return len(q.Items) == 0 }
