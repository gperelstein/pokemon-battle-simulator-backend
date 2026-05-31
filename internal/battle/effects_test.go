package battle_test

import (
	"testing"

	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/battle"
	"github.com/gperelstein/pokemon-battle-simulator-backend/internal/pokemon"
)

// --- helpers específicos del paso 8 ---

// monFull arma un set con ability, item y hasta 4 moves.
func monFull(species string, level int, ability, item string, moves ...string) pokemon.Pokemon {
	p := pokemon.Pokemon{SpeciesID: species, Level: level, AbilityID: ability, ItemID: item}
	for i, m := range moves {
		if i < len(p.Moves) {
			p.Moves[i] = m
		}
	}
	return p
}

// firstDamageTo devuelve el primer monto de daño dirigido a side (o -1).
func firstDamageTo(evs []battle.Event, side battle.SideID) int {
	for _, ev := range evs {
		if ev.Kind == battle.EventDamage && ev.Side == side {
			return ev.Amount
		}
	}
	return -1
}

// hasEvent indica si hay un evento de cierto kind con un Reason dado.
func hasEventReason(evs []battle.Event, kind battle.EventKind, reason string) bool {
	for _, ev := range evs {
		if ev.Kind == kind && ev.Reason == reason {
			return true
		}
	}
	return false
}

// runTurn ejecuta un turno completo (ambos lados usan el slot dado).
func runTurn(t *testing.T, e *battle.Engine, st *battle.State, slotA, slotB int) []battle.Event {
	t.Helper()
	if _, err := e.Apply(st, moveAction(battle.SideA, slotA)); err != nil {
		t.Fatalf("acción A: %v", err)
	}
	evs, err := e.Apply(st, moveAction(battle.SideB, slotB))
	if err != nil {
		t.Fatalf("acción B: %v", err)
	}
	return evs
}

// --- status moves ---

func TestWillOWispBurns(t *testing.T) {
	for seed := uint64(0); seed < 40; seed++ {
		e, st := newBattle(t, seed, team(mon("charizard", "willowisp")), team(mon("pikachu", "tackle")))
		evs := runTurn(t, e, st, 0, 0)
		if count(evs, battle.EventMiss) > 0 {
			continue // Will-O-Wisp falló (acc 85); probamos otro seed
		}
		if active(st, battle.SideB).Status != battle.StatusBurn {
			t.Fatalf("seed %d: status = %q, want brn", seed, active(st, battle.SideB).Status)
		}
		if count(evs, battle.EventStatusInflicted) != 1 {
			t.Errorf("StatusInflicted = %d, want 1", count(evs, battle.EventStatusInflicted))
		}
		return
	}
	t.Fatal("Will-O-Wisp nunca acertó en 40 seeds")
}

func TestWillOWispFireImmune(t *testing.T) {
	// Charizard es Fuego → no se puede quemar, aunque acierte.
	e, st := newBattle(t, 1, team(mon("pikachu", "willowisp")), team(mon("charizard", "tackle")))
	runTurn(t, e, st, 0, 0)
	if active(st, battle.SideB).Status != battle.StatusNone {
		t.Errorf("charizard status = %q, want none (inmune a quemadura)", active(st, battle.SideB).Status)
	}
}

func TestThunderWaveParalyzes(t *testing.T) {
	for seed := uint64(0); seed < 40; seed++ {
		e, st := newBattle(t, seed, team(mon("pikachu", "thunderwave")), team(mon("charizard", "tackle")))
		evs := runTurn(t, e, st, 0, 0)
		if count(evs, battle.EventMiss) > 0 {
			continue
		}
		if active(st, battle.SideB).Status != battle.StatusParalyze {
			t.Fatalf("seed %d: status = %q, want par", seed, active(st, battle.SideB).Status)
		}
		return
	}
	t.Fatal("Thunder Wave nunca acertó en 40 seeds")
}

func TestThunderWaveElectricImmune(t *testing.T) {
	// Pikachu es Electric → inmune a parálisis (gen 6+).
	e, st := newBattle(t, 1, team(mon("charizard", "thunderwave")), team(mon("pikachu", "tackle")))
	runTurn(t, e, st, 0, 0)
	if active(st, battle.SideB).Status != battle.StatusNone {
		t.Errorf("pikachu status = %q, want none (inmune a parálisis)", active(st, battle.SideB).Status)
	}
}

func TestToxicSetsAndScalesInEndOfTurn(t *testing.T) {
	for seed := uint64(0); seed < 40; seed++ {
		e, st := newBattle(t, seed, team(mon("charizard", "toxic")), team(mon("pikachu", "thunderbolt")))
		evs := runTurn(t, e, st, 0, 0)
		if count(evs, battle.EventMiss) > 0 {
			continue
		}
		b := active(st, battle.SideB)
		if b.Status != battle.StatusToxic {
			t.Fatalf("seed %d: status = %q, want tox", seed, b.Status)
		}
		// El primer tick de fin de turno hizo 1/16 (ToxicCount pasó de 0 a 1).
		if b.StatusData.ToxicCount != 1 {
			t.Errorf("ToxicCount = %d, want 1 tras el primer EOT", b.StatusData.ToxicCount)
		}
		if !hasEventReason(evs, battle.EventDamage, "tox") {
			t.Error("falta el daño de tóxico en el fin de turno")
		}
		return
	}
	t.Fatal("Toxic nunca acertó en 40 seeds")
}

// --- boosts ---

func TestSwordsDanceBoostsAttack(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "swordsdance")), team(mon("pikachu", "tackle")))
	evs := runTurn(t, e, st, 0, 0)
	if active(st, battle.SideA).Boosts.Atk != 2 {
		t.Errorf("Atk boost = %d, want +2", active(st, battle.SideA).Boosts.Atk)
	}
	if count(evs, battle.EventBoostChanged) < 1 {
		t.Error("falta EventBoostChanged")
	}
}

func TestCalmMindBoosts(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "calmmind")), team(mon("pikachu", "tackle")))
	runTurn(t, e, st, 0, 0)
	b := active(st, battle.SideA).Boosts
	if b.SpA != 1 || b.SpD != 1 {
		t.Errorf("boosts = SpA %d SpD %d, want +1/+1", b.SpA, b.SpD)
	}
}

// --- clima ---

func TestSunnyDaySetsWeather(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "sunnyday")), team(mon("pikachu", "tackle")))
	evs := runTurn(t, e, st, 0, 0)
	if st.Field.Weather != "sun" {
		t.Errorf("weather = %q, want sun", st.Field.Weather)
	}
	// Se setea en 5 y el tick de fin de turno lo baja a 4.
	if st.Field.WeatherTurns != 4 {
		t.Errorf("weatherTurns = %d, want 4", st.Field.WeatherTurns)
	}
	if count(evs, battle.EventWeatherStarted) != 1 {
		t.Errorf("WeatherStarted = %d, want 1", count(evs, battle.EventWeatherStarted))
	}
}

func TestSandstormSetsWeather(t *testing.T) {
	e, st := newBattle(t, 1, team(mon("charizard", "sandstorm")), team(mon("pikachu", "tackle")))
	runTurn(t, e, st, 0, 0)
	if st.Field.Weather != "sand" {
		t.Errorf("weather = %q, want sand", st.Field.Weather)
	}
}

// --- leech seed ---

func TestLeechSeedDrainsInEndOfTurn(t *testing.T) {
	for seed := uint64(0); seed < 40; seed++ {
		e, st := newBattle(t, seed, team(mon("charizard", "leechseed")), team(mon("pikachu", "thunderbolt")))
		evs := runTurn(t, e, st, 0, 0)
		if count(evs, battle.EventMiss) > 0 {
			continue
		}
		if active(st, battle.SideB).Volatiles["leechseed"] == nil {
			t.Fatalf("seed %d: falta el volátil leechseed en el target", seed)
		}
		if !hasEventReason(evs, battle.EventDamage, "leechseed") {
			t.Error("falta el daño de leech seed en el fin de turno")
		}
		if !hasEventReason(evs, battle.EventHeal, "leechseed") {
			t.Error("falta la curación de leech seed al sembrador")
		}
		return
	}
	t.Fatal("Leech Seed nunca acertó en 40 seeds")
}

func TestLeechSeedFailsVsGrass(t *testing.T) {
	// Venusaur es Grass → Leech Seed no prende.
	e, st := newBattle(t, 1, team(mon("charizard", "leechseed")), team(monFull("venusaur", 50, "", "", "tackle")))
	runTurn(t, e, st, 0, 0)
	if active(st, battle.SideB).Volatiles["leechseed"] != nil {
		t.Error("Leech Seed prendió sobre un Grass (no debería)")
	}
}

// --- abilities ---

func TestLevitateImmuneToGround(t *testing.T) {
	a := team(mon("pikachu", "earthquake"))
	b := team(monFull("bronzong", 50, "levitate", "", "tackle"))
	e, st := newBattle(t, 3, a, b)
	evs := runTurn(t, e, st, 0, 0)
	if count(evs, battle.EventImmune) != 1 {
		t.Fatalf("Immune = %d, want 1 (Levitate vs Earthquake)", count(evs, battle.EventImmune))
	}
	if bz := active(st, battle.SideB); bz.HP != bz.MaxHP {
		t.Errorf("bronzong recibió daño (%d/%d) pese a Levitate", bz.HP, bz.MaxHP)
	}
}

func TestIntimidateLowersFoeAttackOnEntry(t *testing.T) {
	a := team(monFull("gyarados", 50, "intimidate", "", "tackle"))
	b := team(mon("charizard", "tackle"))
	_, st := newBattle(t, 1, a, b)
	if atk := active(st, battle.SideB).Boosts.Atk; atk != -1 {
		t.Errorf("charizard Atk boost = %d, want -1 (Intimidate al entrar)", atk)
	}
	if atk := active(st, battle.SideA).Boosts.Atk; atk != 0 {
		t.Errorf("gyarados Atk boost = %d, want 0", atk)
	}
}

func TestSturdySurvivesLethalFromFull(t *testing.T) {
	// Venusaur lvl 100 con Energy Ball (Grass ×4 vs Golem) lo OHKearía, pero
	// Sturdy lo deja en 1 HP desde HP máximo.
	a := team(monFull("venusaur", 100, "", "", "energyball"))
	b := team(monFull("golem", 50, "sturdy", "", "tackle"))
	e, st := newBattle(t, 1, a, b)
	evs := runTurn(t, e, st, 0, 0)
	g := active(st, battle.SideB)
	if g.Fainted || g.HP != 1 {
		t.Fatalf("golem HP = %d (fainted=%v), want 1 (Sturdy)", g.HP, g.Fainted)
	}
	if count(evs, battle.EventAbilityActivated) < 1 {
		t.Error("falta EventAbilityActivated (Sturdy)")
	}
}

func TestSpeedBoostAtEndOfTurn(t *testing.T) {
	a := team(mon("charizard", "tackle"))
	b := team(monFull("blaziken", 50, "speedboost", "", "tackle"))
	e, st := newBattle(t, 1, a, b)
	runTurn(t, e, st, 0, 0)
	if spe := active(st, battle.SideB).Boosts.Spe; spe != 1 {
		t.Errorf("blaziken Spe boost = %d, want +1 (Speed Boost)", spe)
	}
	if spe := active(st, battle.SideA).Boosts.Spe; spe != 0 {
		t.Errorf("charizard Spe boost = %d, want 0", spe)
	}
}

func TestPranksterGivesStatusMovePriority(t *testing.T) {
	// Sableye (Spe 50) es más lento que Charizard (Spe 100), pero Prankster da
	// +1 de prioridad a Thunder Wave (status) → mueve primero.
	a := team(monFull("sableye", 50, "prankster", "", "thunderwave"))
	b := team(mon("charizard", "tackle"))
	e, st := newBattle(t, 1, a, b)
	evs := runTurn(t, e, st, 0, 0)
	first := evs[firstIndex(evs, battle.EventMoveUsed)]
	if first.Side != battle.SideA {
		t.Errorf("primer MoveUsed Side = %v, want SideA (Prankster)", first.Side)
	}
}

// --- items ---

func TestChoiceBandBoostsDamage(t *testing.T) {
	measure := func(item string) int {
		a := team(monFull("golem", 50, "", item, "tackle"))
		b := team(mon("pikachu", "tackle"))
		e, st := newBattle(t, 7, a, b)
		evs := runTurn(t, e, st, 0, 0)
		return firstDamageTo(evs, battle.SideB)
	}
	plain := measure("")
	band := measure("choiceband")
	if plain <= 0 || band <= 0 {
		t.Fatalf("daños inválidos: plain=%d band=%d", plain, band)
	}
	if band <= plain {
		t.Errorf("daño con Choice Band = %d, want > %d (sin item)", band, plain)
	}
}

func TestChoiceLockRestrictsMove(t *testing.T) {
	a := team(monFull("golem", 50, "", "choiceband", "tackle", "quickattack"))
	b := team(mon("pikachu", "tackle"))
	e, st := newBattle(t, 1, a, b)

	// Turno 1: Golem usa tackle (slot 0) → queda lockeado a ese slot.
	runTurn(t, e, st, 0, 0)
	if st.Turn != 2 {
		t.Fatalf("Turn = %d, want 2", st.Turn)
	}
	// Turno 2: intentar quickattack (slot 1) debe fallar; tackle (slot 0) no.
	if _, err := e.Apply(st, moveAction(battle.SideA, 1)); err != battle.ErrChoiceLocked {
		t.Errorf("quickattack lockeado: err = %v, want ErrChoiceLocked", err)
	}
	if _, err := e.Apply(st, moveAction(battle.SideA, 0)); err != nil {
		t.Errorf("tackle (slot lockeado): err = %v, want nil", err)
	}
}

func TestLifeOrbBoostsDamageAndRecoils(t *testing.T) {
	measure := func(item string) (dmgToB int, evs []battle.Event, st *battle.State) {
		a := team(monFull("charizard", 50, "", item, "tackle"))
		b := team(monFull("golem", 50, "", "", "tackle"))
		e, s := newBattle(t, 7, a, b)
		ev := runTurn(t, e, s, 0, 0)
		return firstDamageTo(ev, battle.SideB), ev, s
	}
	plain, _, _ := measure("")
	orb, evs, st := measure("lifeorb")
	if orb <= plain {
		t.Errorf("daño con Life Orb = %d, want > %d (sin item)", orb, plain)
	}
	if !hasEventReason(evs, battle.EventDamage, "lifeorb") {
		t.Error("falta el retroceso de Life Orb")
	}
	if a := active(st, battle.SideA); a.HP >= a.MaxHP {
		t.Errorf("charizard HP = %d/%d, want < max (retroceso)", a.HP, a.MaxHP)
	}
}

func TestFocusSashSurvivesAndIsConsumed(t *testing.T) {
	a := team(monFull("venusaur", 100, "", "", "energyball"))
	b := team(monFull("golem", 50, "", "focussash", "tackle"))
	e, st := newBattle(t, 1, a, b)
	evs := runTurn(t, e, st, 0, 0)
	g := active(st, battle.SideB)
	if g.Fainted || g.HP != 1 {
		t.Fatalf("golem HP = %d (fainted=%v), want 1 (Focus Sash)", g.HP, g.Fainted)
	}
	if count(evs, battle.EventItemConsumed) != 1 {
		t.Errorf("ItemConsumed = %d, want 1", count(evs, battle.EventItemConsumed))
	}
	if g.Set.ItemID != "" {
		t.Errorf("item = %q, want vacío (consumido)", g.Set.ItemID)
	}
}

// --- secundarios y burn físico ---

func TestFlamethrowerSecondaryBurn(t *testing.T) {
	// Flamethrower 0.5× vs Golem no lo noquea; el secundario (10%) puede quemar.
	for seed := uint64(0); seed < 200; seed++ {
		a := team(mon("charizard", "flamethrower"))
		b := team(monFull("golem", 50, "", "", "tackle"))
		e, st := newBattle(t, seed, a, b)
		runTurn(t, e, st, 0, 0)
		if active(st, battle.SideB).Status == battle.StatusBurn {
			return // el secundario disparó alguna vez → ok
		}
	}
	t.Fatal("el secundario de Flamethrower no quemó en 200 seeds")
}

func TestIceBeamSecondaryFreeze(t *testing.T) {
	for seed := uint64(0); seed < 200; seed++ {
		a := team(monFull("venusaur", 50, "", "", "icebeam"))
		b := team(mon("pikachu", "tackle"))
		e, st := newBattle(t, seed, a, b)
		runTurn(t, e, st, 0, 0)
		if active(st, battle.SideB).Status == battle.StatusFreeze {
			return
		}
	}
	t.Fatal("el secundario de Ice Beam no congeló en 200 seeds")
}

func TestBurnHalvesPhysicalDamage(t *testing.T) {
	measure := func(burned bool) int {
		a := team(monFull("golem", 50, "", "", "tackle"))
		b := team(mon("pikachu", "tackle"))
		e, st := newBattle(t, 7, a, b)
		if burned {
			active(st, battle.SideA).Status = battle.StatusBurn
		}
		evs := runTurn(t, e, st, 0, 0)
		return firstDamageTo(evs, battle.SideB)
	}
	normal := measure(false)
	burned := measure(true)
	if burned >= normal {
		t.Errorf("daño físico quemado = %d, want < %d (normal)", burned, normal)
	}
}
