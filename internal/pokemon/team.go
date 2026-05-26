package pokemon

// Pokemon es una instancia configurada por el usuario: el "set" del Pokémon.
// Vive en el equipo y no se mutila en batalla; las mutaciones de combate van
// en BattlePokemon (paquete battle).
type Pokemon struct {
	SpeciesID string
	Nickname  string
	Level     int          // típicamente 1..100
	Nature    Nature       // ignorada si gen.Rules.HasNatures == false
	IVs       Stats        // 0..31 cada una
	EVs       Stats        // 0..255 cada una, suma <= 510 (Gen 3+)
	AbilityID string       // ignorada si !HasAbilities
	ItemID    string       // ignorado si !HasItems
	Moves     [4]string    // ids de moves; los huecos vacíos son ""
	Gender    string       // "M" | "F" | "N"
	Shiny     bool
	Happiness int          // afecta algunos moves
}

// Team es lo que el usuario lleva a la batalla.
type Team struct {
	Name    string
	Format  string      // "gen9ou", "gen3uu", etc.; informativo en el MVP
	Members [6]Pokemon  // slots vacíos: SpeciesID == ""
}
