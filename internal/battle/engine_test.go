package battle_test

import (
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/battle"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/dex"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/gen"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// --- helpers ---

func loadDex(t *testing.T) dex.Dex {
	t.Helper()
	d, err := dex.Load("../dex/testdata")
	if err != nil {
		t.Fatalf("dex.Load: %v", err)
	}
	return d
}

func mon(species, move string) pokemon.Pokemon {
	return pokemon.Pokemon{SpeciesID: species, Level: 50, Moves: [4]string{move, "", "", ""}}
}

func team(mons ...pokemon.Pokemon) pokemon.Team {
	var tm pokemon.Team
	for i, m := range mons {
		tm.Members[i] = m
	}
	return tm
}

// newBattle arma un Engine + State ya iniciado (post-Start) con los equipos dados.
func newBattle(t *testing.T, seed uint64, a, b pokemon.Team) (*battle.Engine, *battle.State) {
	t.Helper()
	e := battle.New(loadDex(t), gen.For(9))
	st := battle.NewState(9, seed, "test",
		battle.TeamSetup{Player: battle.PlayerInfo{ID: "a", Name: "A"}, Team: a},
		battle.TeamSetup{Player: battle.PlayerInfo{ID: "b", Name: "B"}, Team: b},
	)
	if _, err := e.Start(st); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e, st
}

func moveAction(side battle.SideID, slot int) battle.Action {
	return battle.Action{Kind: battle.ActionMove, Side: side, Move: &battle.MoveAction{MoveSlot: slot}}
}

func switchAction(side battle.SideID, slot int) battle.Action {
	return battle.Action{Kind: battle.ActionSwitch, Side: side, Switch: &battle.SwitchAction{TeamSlot: slot}}
}

func count(evs []battle.Event, kind battle.EventKind) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func firstIndex(evs []battle.Event, kind battle.EventKind) int {
	for i, ev := range evs {
		if ev.Kind == kind {
			return i
		}
	}
	return -1
}

func active(st *battle.State, side battle.SideID) *battle.BattlePokemon {
	s := &st.Sides[side]
	return &s.Team[s.Active]
}

// --- tests ---

func TestStartInitializes(t *testing.T) {
	_, st := newBattle(t, 1, team(mon("charizard", "flamethrower")), team(mon("pikachu", "thunderbolt")))

	if st.Phase != battle.PhaseAwaitingActions {
		t.Errorf("Phase = %v, want AwaitingActions", st.Phase)
	}
	if st.Turn != 1 {
		t.Errorf("Turn = %d, want 1", st.Turn)
	}
	cz := active(st, battle.SideA)
	if cz.MaxHP <= 0 || cz.HP != cz.MaxHP {
		t.Errorf("charizard HP = %d/%d, want HP == MaxHP > 0", cz.HP, cz.MaxHP)
	}
	if cz.Stats.Spe <= 0 {
		t.Errorf("charizard Spe = %d, want > 0", cz.Stats.Spe)
	}
	if cz.PP[0] <= 0 {
		t.Errorf("PP[0] = %d, want > 0 (flamethrower)", cz.PP[0])
	}
}

func TestTurnResolvesFasterFirst(t *testing.T) {
	// Charizard (Spe base 100) es más rápido que Pikachu (Spe base 90).
	// Usan tackle (40 de poder, sin STAB) para que ninguno caiga de un golpe.
	e, st := newBattle(t, 42, team(mon("charizard", "tackle")), team(mon("pikachu", "tackle")))

	if evs, err := e.Apply(st, moveAction(battle.SideA, 0)); err != nil || len(evs) != 0 {
		t.Fatalf("primera acción: evs=%v err=%v (se esperaba esperar al rival)", evs, err)
	}
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("segunda acción: %v", err)
	}

	if count(evs, battle.EventMoveUsed) != 2 {
		t.Fatalf("MoveUsed = %d, want 2", count(evs, battle.EventMoveUsed))
	}
	// El primer move lo usa el lado más rápido (A / charizard).
	first := evs[firstIndex(evs, battle.EventMoveUsed)]
	if first.Side != battle.SideA {
		t.Errorf("primer MoveUsed Side = %v, want SideA (más rápido)", first.Side)
	}
	// Ambos recibieron daño y siguen vivos → turno avanza.
	if active(st, battle.SideA).HP >= active(st, battle.SideA).MaxHP {
		t.Error("charizard no recibió daño")
	}
	if active(st, battle.SideB).HP >= active(st, battle.SideB).MaxHP {
		t.Error("pikachu no recibió daño")
	}
	if st.Turn != 2 || st.Phase != battle.PhaseAwaitingActions {
		t.Errorf("tras el turno: Turn=%d Phase=%v, want 2/AwaitingActions", st.Turn, st.Phase)
	}
}

func TestPriorityOverridesSpeed(t *testing.T) {
	// Pikachu (Spe 90) es más lento, pero Quick Attack (prioridad +1) lo hace
	// mover antes que el Tackle (prioridad 0) de Charizard.
	e, st := newBattle(t, 1, team(mon("charizard", "tackle")), team(mon("pikachu", "quickattack")))

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	first := evs[firstIndex(evs, battle.EventMoveUsed)]
	if first.Side != battle.SideB {
		t.Errorf("primer MoveUsed Side = %v, want SideB (mayor prioridad)", first.Side)
	}
}

func TestSuperEffective(t *testing.T) {
	// Thunderbolt (electric) de Pikachu vs Charizard (fire/flying) → ×2.
	e, st := newBattle(t, 4, team(mon("charizard", "tackle")), team(mon("pikachu", "thunderbolt")))

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, _ := e.Apply(st, moveAction(battle.SideB, 0))

	if count(evs, battle.EventSuperEffective) != 1 {
		t.Fatalf("SuperEffective = %d, want 1", count(evs, battle.EventSuperEffective))
	}
}

func TestNotVeryEffective(t *testing.T) {
	// Flamethrower (fire) vs Charizard (fire/flying) → ×0.5.
	e, st := newBattle(t, 4, team(mon("charizard", "flamethrower")), team(mon("charizard", "tackle")))

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, _ := e.Apply(st, moveAction(battle.SideB, 0))

	if count(evs, battle.EventNotVeryEffective) != 1 {
		t.Fatalf("NotVeryEffective = %d, want 1", count(evs, battle.EventNotVeryEffective))
	}
}

func TestImmunity(t *testing.T) {
	// Earthquake (ground) vs Charizard (flying) → inmune, sin daño.
	e, st := newBattle(t, 4, team(mon("charizard", "tackle")), team(mon("pikachu", "earthquake")))

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, _ := e.Apply(st, moveAction(battle.SideB, 0))

	if count(evs, battle.EventImmune) != 1 {
		t.Fatalf("Immune = %d, want 1", count(evs, battle.EventImmune))
	}
	cz := active(st, battle.SideA)
	if cz.HP != cz.MaxHP {
		t.Errorf("charizard recibió daño (%d/%d) pese a ser inmune", cz.HP, cz.MaxHP)
	}
	for _, ev := range evs {
		if ev.Kind == battle.EventDamage && ev.Side == battle.SideA {
			t.Error("hubo evento de daño sobre el inmune")
		}
	}
}

func TestVoluntarySwitchHappensBeforeMove(t *testing.T) {
	a := team(mon("charizard", "flamethrower"), mon("pikachu", "thunderbolt"))
	b := team(mon("pikachu", "thunderbolt"))
	e, st := newBattle(t, 7, a, b)

	e.Apply(st, switchAction(battle.SideA, 1)) // A cambia a pikachu (slot 1)
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if active(st, battle.SideA) != &st.Sides[battle.SideA].Team[1] {
		t.Fatalf("A.Active = %d, want 1", st.Sides[battle.SideA].Active)
	}
	si, mu := firstIndex(evs, battle.EventSwitchIn), firstIndex(evs, battle.EventMoveUsed)
	if si == -1 || mu == -1 || si > mu {
		t.Errorf("el switch-in (idx %d) debe ocurrir antes del move (idx %d)", si, mu)
	}
	// El daño de B pega sobre el nuevo activo de A (slot 1).
	for _, ev := range evs {
		if ev.Kind == battle.EventDamage && ev.Side == battle.SideA && ev.Slot != 1 {
			t.Errorf("daño sobre slot %d, want 1 (el que entró)", ev.Slot)
		}
	}
}

func TestFaintTriggersForcedSwitch(t *testing.T) {
	a := team(mon("charizard", "flamethrower"))
	b := team(mon("pikachu", "thunderbolt"), mon("charizard", "flamethrower"))
	e, st := newBattle(t, 3, a, b)

	active(st, battle.SideB).HP = 1 // pikachu cae con cualquier golpe

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if count(evs, battle.EventFainted) != 1 {
		t.Fatalf("Fainted = %d, want 1", count(evs, battle.EventFainted))
	}
	if st.Phase != battle.PhaseAwaitingForcedSwitch {
		t.Fatalf("Phase = %v, want AwaitingForcedSwitch", st.Phase)
	}
	if len(st.PendingSwitches) != 1 || st.PendingSwitches[0] != battle.SideB {
		t.Fatalf("PendingSwitches = %v, want [SideB]", st.PendingSwitches)
	}

	// B manda su reemplazo (charizard, slot 1) → vuelve a AwaitingActions, turno 2.
	evs2, err := e.Apply(st, battle.Action{
		Kind: battle.ActionForcedSwitch, Side: battle.SideB,
		Switch: &battle.SwitchAction{TeamSlot: 1},
	})
	if err != nil {
		t.Fatalf("forced switch: %v", err)
	}
	if st.Sides[battle.SideB].Active != 1 {
		t.Errorf("B.Active = %d, want 1", st.Sides[battle.SideB].Active)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("tras forced switch: Phase=%v Turn=%d, want AwaitingActions/2", st.Phase, st.Turn)
	}
	if len(st.PendingSwitches) != 0 {
		t.Errorf("PendingSwitches = %v, want vacío", st.PendingSwitches)
	}
	if count(evs2, battle.EventSwitchIn) != 1 {
		t.Errorf("SwitchIn = %d, want 1", count(evs2, battle.EventSwitchIn))
	}
}

func TestBattleEndsWhenNoReplacement(t *testing.T) {
	a := team(mon("charizard", "flamethrower"))
	b := team(mon("pikachu", "thunderbolt"))
	e, st := newBattle(t, 5, a, b)

	active(st, battle.SideB).HP = 1

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if st.Phase != battle.PhaseEnded {
		t.Fatalf("Phase = %v, want Ended", st.Phase)
	}
	if st.Winner == nil || *st.Winner != battle.SideA {
		t.Fatalf("Winner = %v, want SideA", st.Winner)
	}
	if count(evs, battle.EventBattleEnded) != 1 {
		t.Errorf("BattleEnded = %d, want 1", count(evs, battle.EventBattleEnded))
	}

	// Tras terminar, cualquier acción es rechazada.
	if _, err := e.Apply(st, moveAction(battle.SideA, 0)); err == nil {
		t.Error("se esperaba error al actuar tras el fin de la batalla")
	}
}

func TestForfeit(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "flamethrower")), team(mon("pikachu", "thunderbolt")))

	evs, err := e.Apply(st, battle.Action{Kind: battle.ActionForfeit, Side: battle.SideA})
	if err != nil {
		t.Fatalf("forfeit: %v", err)
	}
	if st.Phase != battle.PhaseEnded || st.Winner == nil || *st.Winner != battle.SideB {
		t.Fatalf("tras forfeit: Phase=%v Winner=%v, want Ended/SideB", st.Phase, st.Winner)
	}
	if count(evs, battle.EventBattleEnded) != 1 {
		t.Errorf("BattleEnded = %d, want 1", count(evs, battle.EventBattleEnded))
	}
}

func TestDeterministicWithSameSeed(t *testing.T) {
	dmg := func(seed uint64) []int {
		a := team(mon("charizard", "tackle"))
		b := team(mon("pikachu", "tackle"))
		e, st := newBattle(t, seed, a, b)
		e.Apply(st, moveAction(battle.SideA, 0))
		evs, _ := e.Apply(st, moveAction(battle.SideB, 0))
		var out []int
		for _, ev := range evs {
			if ev.Kind == battle.EventDamage {
				out = append(out, ev.Amount)
			}
		}
		return out
	}

	a, b := dmg(99), dmg(99)
	if len(a) != 2 || len(a) != len(b) {
		t.Fatalf("daños = %v / %v, want 2 cada uno", a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("mismo seed dio daños distintos: %v vs %v", a, b)
		}
	}
}

func TestUTurnPausesForAttackerSwitch(t *testing.T) {
	// Charizard (más rápido) usa U-turn: pega y luego SU dueño debe elegir relevo
	// a mitad de turno, antes de que B mueva.
	a := team(mon("charizard", "uturn"), mon("pikachu", "tackle"))
	b := team(mon("pikachu", "tackle"))
	e, st := newBattle(t, 11, a, b)

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if st.Phase != battle.PhaseAwaitingForcedSwitch {
		t.Fatalf("Phase = %v, want AwaitingForcedSwitch (pausa de U-turn)", st.Phase)
	}
	if len(st.PendingSwitches) != 1 || st.PendingSwitches[0] != battle.SideA {
		t.Fatalf("PendingSwitches = %v, want [SideA]", st.PendingSwitches)
	}
	// U-turn pegó pero B todavía NO tackleó (su move quedó en la cola).
	if count(evs, battle.EventRequestForcedSwitch) != 1 {
		t.Errorf("RequestForcedSwitch = %d, want 1", count(evs, battle.EventRequestForcedSwitch))
	}
	if count(evs, battle.EventMoveUsed) != 1 {
		t.Errorf("MoveUsed = %d antes de la pausa, want 1 (solo U-turn)", count(evs, battle.EventMoveUsed))
	}

	// A elige a pikachu (slot 1). Se reanuda: B tackle pega al nuevo activo.
	evs2, err := e.Apply(st, battle.Action{Kind: battle.ActionForcedSwitch, Side: battle.SideA, Switch: &battle.SwitchAction{TeamSlot: 1}})
	if err != nil {
		t.Fatalf("forced switch: %v", err)
	}
	if st.Sides[battle.SideA].Active != 1 {
		t.Errorf("A.Active = %d, want 1", st.Sides[battle.SideA].Active)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("tras reanudar: Phase=%v Turn=%d, want AwaitingActions/2", st.Phase, st.Turn)
	}
	// B sí ejecutó su tackle al reanudar.
	if count(evs2, battle.EventMoveUsed) != 1 {
		t.Errorf("MoveUsed al reanudar = %d, want 1 (tackle de B)", count(evs2, battle.EventMoveUsed))
	}
}

func TestUTurnNoReplacementNoPause(t *testing.T) {
	// Con un solo Pokémon, U-turn solo pega: no hay a quién cambiar, no pausa.
	a := team(mon("charizard", "uturn"))
	b := team(mon("pikachu", "tackle"))
	e, st := newBattle(t, 11, a, b)

	e.Apply(st, moveAction(battle.SideA, 0))
	_, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("Phase=%v Turn=%d, want AwaitingActions/2 (sin pausa)", st.Phase, st.Turn)
	}
	if len(st.PendingSwitches) != 0 {
		t.Errorf("PendingSwitches = %v, want vacío", st.PendingSwitches)
	}
}

func TestRoarDragsDefenderRandomly(t *testing.T) {
	// Roar (prioridad -6) saca al activo del rival a un Pokémon al azar del banco.
	a := team(mon("charizard", "roar"))
	b := team(mon("pikachu", "tackle"), mon("charizard", "tackle"))
	e, st := newBattle(t, 8, a, b)

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Único reemplazo posible es el slot 1 → B queda con ese activo. Sin pausa.
	if st.Sides[battle.SideB].Active != 1 {
		t.Errorf("B.Active = %d, want 1 (arrastrado)", st.Sides[battle.SideB].Active)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("Phase=%v Turn=%d, want AwaitingActions/2", st.Phase, st.Turn)
	}
	dragged := false
	for _, ev := range evs {
		if ev.Kind == battle.EventSwitchIn && ev.Side == battle.SideB && ev.Reason == "drag" {
			dragged = true
		}
	}
	if !dragged {
		t.Error("falta el SwitchIn con reason \"drag\"")
	}
}

func TestDragonTailDamagesAndDrags(t *testing.T) {
	// Dragon Tail pega y arrastra. Buscamos un seed donde acierte (accuracy 90).
	for seed := uint64(0); seed < 20; seed++ {
		a := team(mon("charizard", "dragontail"))
		b := team(mon("pikachu", "tackle"), mon("charizard", "tackle"))
		e, st := newBattle(t, seed, a, b)
		e.Apply(st, moveAction(battle.SideA, 0))
		evs, _ := e.Apply(st, moveAction(battle.SideB, 0))

		if count(evs, battle.EventMiss) > 0 {
			continue // falló por accuracy; probamos otro seed
		}
		// Acertó: debe haber pegado a B y haberlo arrastrado al slot 1.
		damagedB := false
		for _, ev := range evs {
			if ev.Kind == battle.EventDamage && ev.Side == battle.SideB {
				damagedB = true
			}
		}
		if !damagedB {
			t.Fatalf("seed %d: Dragon Tail no pegó a B", seed)
		}
		if st.Sides[battle.SideB].Active != 1 {
			t.Fatalf("seed %d: B.Active = %d, want 1 (arrastrado tras el daño)", seed, st.Sides[battle.SideB].Active)
		}
		return // ok
	}
	t.Fatal("ningún seed produjo un acierto de Dragon Tail en 20 intentos")
}

func TestForceSwitchFailsWithoutReplacement(t *testing.T) {
	// Roar contra un rival con un solo Pokémon: no pasa nada (no hay banco).
	a := team(mon("charizard", "roar"))
	b := team(mon("pikachu", "tackle"))
	e, st := newBattle(t, 8, a, b)

	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if st.Sides[battle.SideB].Active != 0 {
		t.Errorf("B.Active = %d, want 0 (no hay a quién arrastrar)", st.Sides[battle.SideB].Active)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("Phase=%v Turn=%d, want AwaitingActions/2", st.Phase, st.Turn)
	}
	for _, ev := range evs {
		if ev.Kind == battle.EventSwitchIn && ev.Reason == "drag" {
			t.Error("no debería haber drag sin reemplazo")
		}
	}
}

func TestInvalidChoices(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "flamethrower")), team(mon("pikachu", "thunderbolt")))

	if _, err := e.Apply(st, moveAction(battle.SideA, 1)); err != battle.ErrInvalidMoveSlot {
		t.Errorf("move a slot vacío: err = %v, want ErrInvalidMoveSlot", err)
	}
	if _, err := e.Apply(st, switchAction(battle.SideA, 1)); err != battle.ErrInvalidSwitchSlot {
		t.Errorf("switch a slot vacío: err = %v, want ErrInvalidSwitchSlot", err)
	}
	// Forced switch fuera de fase.
	fs := battle.Action{Kind: battle.ActionForcedSwitch, Side: battle.SideA, Switch: &battle.SwitchAction{TeamSlot: 1}}
	if _, err := e.Apply(st, fs); err != battle.ErrWrongPhase {
		t.Errorf("forced switch en AwaitingActions: err = %v, want ErrWrongPhase", err)
	}
}
