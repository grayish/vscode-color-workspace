package color

// LadderSteps are the lightness deltas (HSL %) used by LadderOffset. The
// list intentionally excludes 0 — the main worktree's IdentityHash is 0
// by convention, so its offset is always 0% (handled in LadderOffset).
var LadderSteps = []float64{-15, -10, -5, +5, +10, +15}

// LadderOffset maps an identity hash to a lightness delta.
//   - hash == 0  → 0  (main worktree convention)
//   - hash != 0  → LadderSteps[hash % len(LadderSteps)]
func LadderOffset(hash uint64) float64 {
	if hash == 0 {
		return 0
	}
	return LadderSteps[hash%uint64(len(LadderSteps))]
}

// ApplyLightness returns c with HSL lightness shifted by deltaPct percent.
// Delegates to existing Lighten/Darken primitives, which clamp HSL to [0, 1].
func (c Color) ApplyLightness(deltaPct float64) Color {
	switch {
	case deltaPct > 0:
		return c.Lighten(deltaPct)
	case deltaPct < 0:
		return c.Darken(-deltaPct)
	default:
		return c
	}
}
