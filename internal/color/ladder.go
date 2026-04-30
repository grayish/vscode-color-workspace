package color

// LadderRange is the maximum HSL lightness delta (%) applied to derived
// colors. LadderOffset returns an integer offset in [-LadderRange, +LadderRange]
// excluding 0 (which is reserved for the main worktree by convention —
// IdentityHash returns 0 for the main worktree).
const LadderRange = 7

// LadderOffset maps an identity hash to a lightness delta percent.
//   - hash == 0  → 0  (main worktree)
//   - hash != 0  → integer in {-7,…,-1, +1,…,+7}, derived as
//     hash % (2*LadderRange) mapped onto the symmetric set
func LadderOffset(hash uint64) float64 {
	if hash == 0 {
		return 0
	}
	n := int(hash%uint64(2*LadderRange)) - LadderRange // n ∈ [-7, 6]
	if n >= 0 {
		return float64(n + 1) // +1..+7
	}
	return float64(n) // -7..-1
}

// ApplyLightness returns c with HSL lightness shifted by deltaPct percent.
// Delegates to existing Lighten/Darken primitives, which clamp HSL to [0, 1].
// deltaPct == 0 returns c unchanged.
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
