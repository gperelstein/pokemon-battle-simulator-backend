package battle_test

import (
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/battle"
)

// damageWithReason busca el primer evento de daño de un lado con la razón dada.
func damageWithReason(evs []battle.Event, side battle.SideID, reason string) (int, bool) {
	for _, ev := range evs {
		if ev.Kind == battle.EventDamage && ev.Side == side && ev.Reason == reason {
			return ev.Amount, true
		}
	}
	return 0, false
}

func healWithReason(evs []battle.Event, side battle.SideID, reason string) (int, bool) {
	for _, ev := range evs {
		if ev.Kind == battle.EventHeal && ev.Side == side && ev.Reason == reason {
			return ev.Amount, true
		}
	}
	return 0, false
}

// playTurn hace que ambos lados usen su move slot 0 y devuelve los eventos del
// turno completo (incluido el fin de turno).
func playTurn(t *testing.T, e *battle.Engine, st *battle.State) []battle.Event {
	t.Helper()
	e.Apply(st, moveAction(battle.SideA, 0))
	evs, err := e.Apply(st, moveAction(battle.SideB, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return evs
}

func TestBurnResidual(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "tackle")), team(mon("pikachu", "tackle")))
	cz := active(st, battle.SideA)
	cz.Status = battle.StatusBurn

	evs := playTurn(t, e, st)

	amt, ok := damageWithReason(evs, battle.SideA, "brn")
	if !ok {
		t.Fatal("falta el daño de quemadura")
	}
	if amt != cz.MaxHP/16 {
		t.Errorf("daño de quemadura = %d, want maxHP/16 = %d", amt, cz.MaxHP/16)
	}
}

func TestToxicResidualIncreases(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "tackle")), team(mon("pikachu", "tackle")))
	cz := active(st, battle.SideA)
	cz.Status = battle.StatusToxic

	evs1 := playTurn(t, e, st)
	amt1, ok1 := damageWithReason(evs1, battle.SideA, "tox")
	evs2 := playTurn(t, e, st)
	amt2, ok2 := damageWithReason(evs2, battle.SideA, "tox")

	if !ok1 || !ok2 {
		t.Fatalf("faltan daños de tóxico (t1=%v t2=%v)", ok1, ok2)
	}
	if amt1 != cz.MaxHP/16 {
		t.Errorf("tóxico turno 1 = %d, want maxHP*1/16 = %d", amt1, cz.MaxHP/16)
	}
	if amt2 != cz.MaxHP*2/16 {
		t.Errorf("tóxico turno 2 = %d, want maxHP*2/16 = %d", amt2, cz.MaxHP*2/16)
	}
	if cz.StatusData.ToxicCount != 2 {
		t.Errorf("ToxicCount = %d, want 2", cz.StatusData.ToxicCount)
	}
}

func TestLeftoversHeals(t *testing.T) {
	a := team(mon("charizard", "tackle"))
	a.Members[0].ItemID = "leftovers"
	e, st := newBattle(t, 1, a, team(mon("pikachu", "tackle")))
	cz := active(st, battle.SideA)
	cz.HP = cz.MaxHP / 2 // dañado, para que Leftovers tenga efecto

	evs := playTurn(t, e, st)

	amt, ok := healWithReason(evs, battle.SideA, "leftovers")
	if !ok {
		t.Fatal("falta la curación de Leftovers")
	}
	if amt != cz.MaxHP/16 {
		t.Errorf("Leftovers = %d, want maxHP/16 = %d", amt, cz.MaxHP/16)
	}
}

func TestSandstormDamagesNonImmune(t *testing.T) {
	// Golem (rock/ground) es inmune a la arena; Charizard no.
	e, st := newBattle(t, 1, team(mon("golem", "tackle")), team(mon("charizard", "tackle")))
	st.Field.Weather = "sand"
	st.Field.WeatherTurns = 5

	evs := playTurn(t, e, st)

	if _, ok := damageWithReason(evs, battle.SideA, "sand"); ok {
		t.Error("Golem (rock/ground) no debería recibir daño de arena")
	}
	if _, ok := damageWithReason(evs, battle.SideB, "sand"); !ok {
		t.Error("Charizard debería recibir daño de arena")
	}
	if st.Field.WeatherTurns != 4 {
		t.Errorf("WeatherTurns = %d, want 4 (descontó uno)", st.Field.WeatherTurns)
	}
}

func TestWeatherEnds(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "tackle")), team(mon("pikachu", "tackle")))
	st.Field.Weather = "rain"
	st.Field.WeatherTurns = 1 // termina este fin de turno

	evs := playTurn(t, e, st)

	if st.Field.Weather != "" {
		t.Errorf("Weather = %q, want vacío (terminó)", st.Field.Weather)
	}
	ended := false
	for _, ev := range evs {
		if ev.Kind == battle.EventWeatherEnded && ev.Reason == "rain" {
			ended = true
		}
	}
	if !ended {
		t.Error("falta el evento WeatherEnded")
	}
}

func TestLeechSeedDrainsAndHeals(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "tackle")), team(mon("pikachu", "tackle")))
	seeded := active(st, battle.SideA)
	seeded.Volatiles["leechseed"] = true
	healer := active(st, battle.SideB)
	healer.HP = healer.MaxHP / 2 // dañado, para ver la curación

	evs := playTurn(t, e, st)

	drain, okD := damageWithReason(evs, battle.SideA, "leechseed")
	heal, okH := healWithReason(evs, battle.SideB, "leechseed")
	if !okD {
		t.Fatal("falta el drenado de Leech Seed sobre A")
	}
	if drain != seeded.MaxHP/8 {
		t.Errorf("drenado = %d, want maxHP/8 = %d", drain, seeded.MaxHP/8)
	}
	if !okH {
		t.Fatal("falta la curación de Leech Seed sobre B")
	}
	if heal != drain {
		t.Errorf("curación = %d, want = drenado = %d", heal, drain)
	}
}

func TestResidualFaintTriggersForcedSwitch(t *testing.T) {
	// A (tóxico, 1 HP) sobrevive la fase de moves (B cambia, no ataca) y cae por
	// el residual de fin de turno → forced switch.
	a := team(mon("charizard", "tackle"), mon("pikachu", "tackle"))
	b := team(mon("pikachu", "tackle"), mon("charizard", "tackle"))
	e, st := newBattle(t, 1, a, b)
	cz := active(st, battle.SideA)
	cz.Status = battle.StatusToxic
	cz.HP = 1

	e.Apply(st, moveAction(battle.SideA, 0))               // A ataca
	evs, err := e.Apply(st, switchAction(battle.SideB, 1)) // B cambia (no pega a A)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !cz.Fainted {
		t.Fatal("A debería caer por el tóxico de fin de turno")
	}
	if st.Phase != battle.PhaseAwaitingForcedSwitch || len(st.PendingSwitches) != 1 || st.PendingSwitches[0] != battle.SideA {
		t.Fatalf("Phase=%v Pending=%v, want AwaitingForcedSwitch [SideA]", st.Phase, st.PendingSwitches)
	}
	if _, ok := damageWithReason(evs, battle.SideA, "tox"); !ok {
		t.Error("falta el daño de tóxico que lo noqueó")
	}

	// A manda reemplazo → próximo turno.
	if _, err := e.Apply(st, battle.Action{Kind: battle.ActionForcedSwitch, Side: battle.SideA, Switch: &battle.SwitchAction{TeamSlot: 1}}); err != nil {
		t.Fatalf("forced switch: %v", err)
	}
	if st.Phase != battle.PhaseAwaitingActions || st.Turn != 2 {
		t.Errorf("Phase=%v Turn=%d, want AwaitingActions/2", st.Phase, st.Turn)
	}
}
