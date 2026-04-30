# Narrow Worktree Color Family Range Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Narrow the worktree-derived lightness offset from the existing 6-bucket `LadderSteps = {-15, -10, -5, +5, +10, +15}` to a 14-value `±1..±7` range so worktree colors feel like a tight family.

**Architecture:** Replace the `LadderSteps` slice in `internal/color/ladder.go` with a `LadderRange = 7` constant. Rewrite `LadderOffset` to map `hash % (2*LadderRange)` onto the symmetric set `{-7,…,-1, +1,…,+7}`. Update tests to assert the new range; sync the README's family-range description.

**Tech Stack:** Go 1.23+, project Taskfile-based build (`task`, `task lint`, `task test:race`).

**Spec:** `docs/superpowers/specs/2026-04-27-worktree-similar-color-design.md` §7

---

## Pre-flight

The worktree-color implementation lives on the not-yet-merged branch `feat/worktree-similar-color`. The narrowing change should land on top of that branch.

```bash
git checkout feat/worktree-similar-color
git status                         # working tree clean
git log --oneline -5               # confirm tip is the existing worktree-color work
task test                          # baseline: existing ±15 implementation passes
```

If `task test` fails on the baseline, stop and investigate before continuing.

---

### Task 1: Narrow `LadderOffset` to ±7 with 14 buckets

**Files:**
- Modify: `internal/color/ladder.go` (rewrite — file is ~30 lines)
- Modify: `internal/color/ladder_test.go` (rewrite — `LadderSteps` references must go)

- [ ] **Step 1: Rewrite `internal/color/ladder_test.go`**

Replace the entire file contents (the existing tests reference `LadderSteps`, which is being deleted):

```go
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
```

Notes on what changed vs the existing file:
- Removed `TestLadderSteps_NoZero` — the slice no longer exists; the equivalent guarantee (offset never 0) is covered by `TestLadderOffset_NonZeroInRange`.
- `TestLadderOffset_NonZeroInRange` no longer iterates `LadderSteps`; it asserts integer-in-`[-LadderRange, LadderRange]`-and-not-zero directly.
- Added `TestLadderOffset_AllValuesReachable` — sweeps hashes and verifies each of the 14 expected values appears.
- Added `TestLadderOffset_Distribution` — checks rough uniform spread over the 14 buckets.
- `TestLadderOffset_Stable` and the three `TestApplyLightness_*` tests carry over unchanged.

- [ ] **Step 2: Run the new tests — expect compile failure**

```bash
go test ./internal/color/ -run TestLadderOffset -v
```

Expected: package fails to compile because `LadderRange` is referenced but not yet defined (still `LadderSteps` in `ladder.go`). The error message will mention `undefined: LadderRange`.

- [ ] **Step 3: Rewrite `internal/color/ladder.go`**

Replace the entire file:

```go
package color

// LadderRange is the maximum HSL lightness delta (%) applied to derived
// colors. LadderOffset returns an integer offset in [-LadderRange, +LadderRange]
// excluding 0 (which is reserved for the main worktree by convention —
// IdentityHash returns 0 for the main worktree).
const LadderRange = 7

// LadderOffset maps an identity hash to a lightness delta percent.
//   - hash == 0  → 0  (main worktree)
//   - hash != 0  → integer in {-7,…,-1, +1,…,+7}, derived as
//                  hash % (2*LadderRange) mapped onto the symmetric set
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
```

- [ ] **Step 4: Run ladder tests — expect pass**

```bash
go test ./internal/color/ -run "Test(LadderOffset|ApplyLightness)" -v
```

Expected: 8 tests pass — `TestLadderOffset_ZeroHashIsZero`, `_NonZeroInRange`, `_AllValuesReachable`, `_Distribution`, `_Stable`, plus `TestApplyLightness_PositiveLightens`, `_NegativeDarkens`, `_ZeroIsNoop`.

- [ ] **Step 5: Run full color package — guard against golden/parity regressions**

```bash
go test ./internal/color/ -v
```

Expected: pass. (The Peacock-parity golden tests don't touch `LadderOffset`; this is a safety net.)

- [ ] **Step 6: Run runner tests — `LadderOffset` is consumed in `resolveFromWorktree`**

```bash
go test ./internal/runner/ -v
```

Expected: pass. The Case A/C tests in `resolve_test.go` use relative assertions (e.g., `linked != base`, `warns contains "#…"`) rather than hardcoded post-offset hex, so they remain valid under the new range.

- [ ] **Step 7: Lint + race**

```bash
task lint
task test:race
```

Expected: both pass.

- [ ] **Step 8: Commit**

```bash
git add internal/color/ladder.go internal/color/ladder_test.go
git commit -m "$(cat <<'EOF'
color: narrow LadderOffset range to ±7 with 14 buckets

Replace LadderSteps slice ({-15,-10,-5,+5,+10,+15}, 6 buckets) with a
LadderRange = 7 constant and rewrite LadderOffset to pick from
{-7,…,-1, +1,…,+7} via hash % 14.

Effect: max delta between two worktrees drops from 30%p to 14%p,
strengthening the family feel. 5-worktree no-collision probability rises
from ~31% (6 buckets) to ~67% (14 buckets, birthday formula).

Spec: docs/superpowers/specs/2026-04-27-worktree-similar-color-design.md §7

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Sync `README.md` family-range description

**Files:**
- Modify: `README.md` (the "Worktree color family" section, around line 48)

- [ ] **Step 1: Locate the stale text**

```bash
grep -n "±5/±10/±15" README.md
```

Expected output:
```
48:When you run `ccws` inside a git worktree, it automatically picks a "family" color so sibling worktrees of the same repo look related but distinct (same hue/saturation, lightness shifted by ±5/±10/±15%).
```

- [ ] **Step 2: Edit the line**

In `README.md`, replace `±5/±10/±15%` on that line with `±1 to ±7%`. After the edit the line reads:

```
When you run `ccws` inside a git worktree, it automatically picks a "family" color so sibling worktrees of the same repo look related but distinct (same hue/saturation, lightness shifted by ±1 to ±7%).
```

- [ ] **Step 3: Verify no stale numbers remain**

```bash
grep -nE "±5|±10|±15" README.md CLAUDE.md || echo "[clean]"
```

Expected: `[clean]`. (`CLAUDE.md` is grepped as a safety check; it should already be clean.)

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs: README family range now ±1~±7 (was ±5/±10/±15)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Final verification

**Files:** none

- [ ] **Step 1: CI-equivalent check**

```bash
task ci
```

Expected: pass (= `task lint` + `task test:race`).

- [ ] **Step 2: Optional manual smoke**

Skip if no scratch repo handy. To eyeball the family feel:

```bash
mkdir -p /tmp/ccws-smoke && cd /tmp/ccws-smoke
rm -rf main linked-a linked-b
git init main
(cd main && git commit --allow-empty -m init && \
 git worktree add ../linked-a && \
 git worktree add ../linked-b)

ccws main         # main → random anchor, .code-workspace written
ccws linked-a     # linked → anchor.ApplyLightness(±1..±7)
ccws linked-b     # linked → different offset (probably)

cat main/../main.code-workspace            | grep peacock.color
cat linked-a/../linked-a.code-workspace    | grep peacock.color
cat linked-b/../linked-b.code-workspace    | grep peacock.color
```

Visual sanity: the three `peacock.color` hex values should share hue/saturation and differ only in a small lightness shift.

- [ ] **Step 3: Confirm branch state**

```bash
git log --oneline -5
git status
```

Expected: two new commits on top of the prior `feat/worktree-similar-color` tip; clean working tree.

---

## Out of scope (do not do here)

- Rebuild or revisit `internal/gitworktree/` — this plan does not touch worktree discovery.
- Changes to `cmd/ccws/render.go`, `internal/runner/runner.go`, or `Worktree`/`AnchorIntent` types — those are part of the prior plan and remain unchanged.
- Adding a `--ladder-range` flag or other escape hatch — explicit non-goal in spec §1.
- Updating `docs/superpowers/specs/2026-04-27-worktree-similar-color-design.md` — already updated in commit `9ba9b9c` on `main`.
