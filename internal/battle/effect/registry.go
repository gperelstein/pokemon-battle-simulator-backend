// Package effect contiene la lógica concreta de cada movimiento, habilidad e
// ítem que tenga comportamiento más allá del daño base. Los efectos se
// registran por id (mismo id que en el dex / dataset de Showdown) y el motor
// los invoca por nombre durante la resolución.
//
// Diseño:
//   - Un MoveEffect recibe el contexto de ejecución del move (atacante,
//     defensor, state, queue, rng) y muta el state / encola eventos. Devuelve
//     un MoveResult con flags como "userSwitches" (U-turn) o "targetSwitches"
//     (Whirlwind), que el motor lee para encolar QueueRequestForcedSwitch.
//   - Las abilities tienen "hooks" por evento (onSwitchIn, onBeforeMove,
//     onModifyDamage, onFaint, etc.). El motor las dispara en los puntos
//     correspondientes.
//   - Items: ídem abilities.
//
// La firma exacta de Context y los hooks se completarán cuando se implemente
// el primer move real. Por ahora se deja el shape para evitar ciclos de
// dependencias con el paquete battle.
package effect

// Registry contiene las implementaciones de moves/abilities/items por id.
// Lookups O(1). Ids desconocidos devuelven (zero, false) y el motor cae al
// comportamiento por defecto.
type Registry struct {
	moves     map[string]MoveEffect
	abilities map[string]AbilityHooks
	items     map[string]ItemHooks
}

// MoveEffect, AbilityHooks, ItemHooks son interfaces marcador por ahora; sus
// métodos se definen al cablear el primer caso real, para no acoplar este
// paquete con battle antes de tiempo.
type MoveEffect interface{ moveEffect() }
type AbilityHooks interface{ abilityHooks() }
type ItemHooks interface{ itemHooks() }

func New() *Registry {
	return &Registry{
		moves:     map[string]MoveEffect{},
		abilities: map[string]AbilityHooks{},
		items:     map[string]ItemHooks{},
	}
}

func (r *Registry) RegisterMove(id string, e MoveEffect)       { r.moves[id] = e }
func (r *Registry) RegisterAbility(id string, h AbilityHooks)  { r.abilities[id] = h }
func (r *Registry) RegisterItem(id string, h ItemHooks)        { r.items[id] = h }

func (r *Registry) Move(id string) (MoveEffect, bool)       { e, ok := r.moves[id]; return e, ok }
func (r *Registry) Ability(id string) (AbilityHooks, bool)  { h, ok := r.abilities[id]; return h, ok }
func (r *Registry) Item(id string) (ItemHooks, bool)        { h, ok := r.items[id]; return h, ok }
