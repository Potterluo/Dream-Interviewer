package interview

// presets.go: helpers over the compiled-in preset set.
//
// The built-in presets are the curated product surface and are read-only;
// an admin's own presets live in the database. The server concatenates the
// two, so it needs to answer "is this id one of the curated ones?" without
// guessing from the id's shape.

// IsBuiltinPreset reports whether id belongs to the compiled-in set. The
// server uses this to refuse edits with a 409 instead of pretending to
// succeed and then losing the change on the next build.
func IsBuiltinPreset(id string) bool {
	for _, p := range Presets() {
		if p.ID == id {
			return true
		}
	}
	return false
}

// PresetByID looks a built-in preset up, for tests and for explaining an
// existing interview.
func PresetByID(id string) (Preset, bool) {
	for _, p := range Presets() {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}
