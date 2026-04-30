package color

import "testing"

func TestLadderOffset_ZeroHashIsZero(t *testing.T) {
	if got := LadderOffset(0); got != 0 {
		t.Errorf("LadderOffset(0) = %v, want 0", got)
	}
}

func TestLadderOffset_NonZeroInRange(t *testing.T) {
	for hash := uint64(1); hash <= 1000; hash++ {
		got := LadderOffset(hash)
		if got == 0 {
			t.Fatalf("LadderOffset(%d) = 0; 0 is reserved for the main worktree", hash)
		}
		n := int(got)
		if float64(n) != got {
			t.Fatalf("LadderOffset(%d) = %v; want integer", hash, got)
		}
		if n < -LadderRange || n > LadderRange {
			t.Fatalf("LadderOffset(%d) = %d; want in [-%d, %d]", hash, n, LadderRange, LadderRange)
		}
	}
}

func TestLadderOffset_AllValuesReachable(t *testing.T) {
	seen := map[float64]bool{}
	for hash := uint64(1); hash <= 1000; hash++ {
		seen[LadderOffset(hash)] = true
	}
	want := 2 * LadderRange // 14: ±1..±7 excluding 0
	if len(seen) != want {
		t.Errorf("got %d distinct offsets, want %d", len(seen), want)
	}
	for n := -LadderRange; n <= LadderRange; n++ {
		if n == 0 {
			continue
		}
		if !seen[float64(n)] {
			t.Errorf("offset %d never reached over hash 1..1000", n)
		}
	}
}

func TestLadderOffset_Distribution(t *testing.T) {
	counts := map[float64]int{}
	const N = 1000
	for hash := uint64(1); hash <= N; hash++ {
		counts[LadderOffset(hash)]++
	}
	// Expected per bucket: 1000/14 ≈ 71. Allow [50, 100] (~30% slack).
	for off, c := range counts {
		if c < 50 || c > 100 {
			t.Errorf("offset %v: %d hits in 1000-sweep, want in [50, 100]", off, c)
		}
	}
}

func TestLadderOffset_Stable(t *testing.T) {
	for hash := uint64(1); hash < 100; hash++ {
		a := LadderOffset(hash)
		b := LadderOffset(hash)
		if a != b {
			t.Errorf("LadderOffset(%d) not stable: %v vs %v", hash, a, b)
		}
	}
}

func TestApplyLightness_PositiveLightens(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140} // #5a3b8c, L≈39%
	lighter := base.ApplyLightness(10)
	if lighter == base {
		t.Error("ApplyLightness(+10) returned base unchanged")
	}
	if int(lighter.R)+int(lighter.G)+int(lighter.B) <= int(base.R)+int(base.G)+int(base.B) {
		t.Errorf("ApplyLightness(+10) did not lighten: base=%v lighter=%v", base, lighter)
	}
}

func TestApplyLightness_NegativeDarkens(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140}
	darker := base.ApplyLightness(-10)
	if darker == base {
		t.Error("ApplyLightness(-10) returned base unchanged")
	}
	if int(darker.R)+int(darker.G)+int(darker.B) >= int(base.R)+int(base.G)+int(base.B) {
		t.Errorf("ApplyLightness(-10) did not darken: base=%v darker=%v", base, darker)
	}
}

func TestApplyLightness_ZeroIsNoop(t *testing.T) {
	base := Color{R: 90, G: 59, B: 140}
	if got := base.ApplyLightness(0); got != base {
		t.Errorf("ApplyLightness(0) = %v, want %v", got, base)
	}
}
