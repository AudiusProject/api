package jobs

// validMusicalKeys mirrors the apps indexer's MusicalKey enum
// (discovery-provider src/tasks/metadata.py). Note: flats only, no sharps —
// "C# minor" is NOT a valid key; the equivalent is "D flat minor".
var validMusicalKeys = map[string]bool{
	"A major": true, "A minor": true,
	"B flat major": true, "B flat minor": true,
	"B major": true, "B minor": true,
	"C major": true, "C minor": true,
	"D flat major": true, "D flat minor": true,
	"D major": true, "D minor": true,
	"E flat major": true, "E flat minor": true,
	"E major": true, "E minor": true,
	"F major": true, "F minor": true,
	"G flat major": true, "G flat minor": true,
	"G major": true, "G minor": true,
	"A flat major": true, "A flat minor": true,
	"Silence": true,
}

// isValidMusicalKey reports whether s is one of the recognized musical keys.
// Mirrors apps' is_valid_musical_key (src/tasks/metadata.py).
func isValidMusicalKey(s string) bool {
	return validMusicalKeys[s]
}
