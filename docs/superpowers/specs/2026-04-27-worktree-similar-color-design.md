# Worktree similar color — Design

## 1. 문제와 목표

오늘 `ccws`의 색 결정은 `--color > .vscode/settings.json의 peacock.color > 랜덤` 우선순위를 따른다 (`internal/runner/resolve.go`). git 워크트리를 쓰는 사용자는 한 repo에서 여러 워크트리(예: `myproj/`, `myproj-feat-x/`, `myproj-bugfix/`)를 동시에 열어두는데, 각 워크트리가 별도 `.code-workspace`를 갖고 색이 독립적으로 랜덤 결정되므로 **서로 무관한 색**으로 보인다. 같은 repo의 워크트리들이라는 시각적 단서가 사라진다.

목표: 같은 repo의 워크트리들이 자동으로 **같은 색 가족**(같은 hue/saturation, 명도만 살짝 다름)으로 보이도록 만든다. 사용자는 추가 플래그를 외울 필요가 없다. main 워크트리의 색이 anchor가 되고, linked 워크트리들은 그 anchor에서 명도만 ±5/±10/±15% 정도 다른 색을 자동으로 부여받는다.

이 변경은 `2026-04-25-ccws-vscode-color-workspace-design.md`의 §색 결정 우선순위 부분을 확장한다 (supersede가 아닌 추가).

Non-goals:
- 비-워크트리 환경에서 색 유사화 (예: 같은 repo의 별도 클론 두 개)
- 인터랙티브 모드에서 anchor 색 미리보기/선택 UI
- hash 결과를 사용자가 직접 바꿀 수 있는 옵션 (escape hatch는 기존 `--color`로 충분)
- main 워크트리 외부의 `.vscode/settings.json` 정리 (target이 아닌 디렉토리 손대는 것은 invasive)
- bare repo, 6개 이상의 워크트리에서의 충돌 회피 로직

## 2. 트리거와 새 동작 매트릭스

"target이 git 워크트리 안에 있다"의 정의: `git worktree list --porcelain`이 비-에러로 반환되고, target 절대 경로가 그 결과 안의 어느 한 worktree path 아래에 있음.

T = target 워크트리, M = main 워크트리, L = linked 워크트리(들).

| 케이스 | 조건 | 동작 |
|---|---|---|
| **A** | M의 `<parent>/<dirname>.code-workspace`에 peacock.color 있음 | M의 색을 anchor로 채택. T 색 = anchor에 `LadderOffset(IdentityHash(T))` 적용 (T==M이면 오프셋 0). `SourceWorktree` |
| **B** | M에 색 없음 + 다른 모든 워크트리(L 포함)에 색 없음 + T == M | 워크트리 로직 skip. 기존 흐름 (T의 settings.json → 랜덤). `SourceSettings` 또는 `SourceRandom` |
| **C** | M에 색 없음 + 다른 모든 워크트리에 색 없음 + T ∈ L (즉 첫 ccws가 linked) | Random anchor 생성. **M의 `<parent>/<dirname>.code-workspace`를 자동 작성** (peacock 키만 머지, 기존 사용자 설정 보존). T 색 = anchor에 `LadderOffset(IdentityHash(T))` 적용. `SourceWorktree`. `Result.Warnings`에 anchor 자동 생성 알림. |
| **D** | M에 색 없음 + L 중 누군가 색 있음 (T가 main이든 linked든) | 사용자가 명시적으로 만든 비표준 상태로 간주. **family 포기**. `Result.Warnings`에 충돌 알림. 색은 기존 흐름으로 결정 (T의 settings.json → 랜덤) |
| 비-git 디렉토리, git 부재, `git worktree list` 실패 | — | 워크트리 로직 silent skip. 기존 흐름 그대로 |

`--color` 플래그가 명시된 경우 워크트리 로직은 항상 우회된다. T가 linked인 경우 그 색이 그대로 T에 적용되며 오프셋은 붙지 않는다 (사용자가 정확히 그 색을 원한 것으로 간주).

Case C의 "M의 워크스페이스 자동 작성"은 target 외부 디렉토리에 파일을 만드는 사이드 이펙트이므로 **warn 배지로 명시적으로 알린다**. M의 `.vscode/settings.json`은 건드리지 않는다.

## 3. 우선순위 체인

```
1. --color 플래그              → SourceFlag
2. 워크트리 로직 (Case A 또는 C) → SourceWorktree
3. T의 .vscode/settings.json    → SourceSettings  (Case B의 부분 또는 Case D fallback)
4. color.Random()              → SourceRandom    (Case B의 부분 또는 Case D fallback)
```

Case D는 새 우선순위 단계가 아니라 "워크트리 로직이 색을 결정하지 않고 fallback을 트리거하면서 warning만 추가"하는 동작이다.

## 4. 렌더링된 모양

**Case A (모든 worktree 색칠):** 출력 변화 없음. 기존 `ok workspace ready` 배지.

**Case C (linked에서 첫 ccws — anchor 자동 생성):**
```
  warn  family anchor created for main worktree
        anchor at  ~/code/myproj.code-workspace
        applied    ~/code/myproj-feat-x.code-workspace
        hint       run ccws on main worktree to claim color directly
  ok    workspace ready
        ~/code/myproj-feat-x.code-workspace  #6747a4
```

warn 배지가 ok 배지보다 위에 와서, 사이드 이펙트(외부 파일 생성)를 사용자가 먼저 인지하도록 한다. stderr.

**Case D (family disabled):**
```
  warn  worktree family disabled
        reason     main worktree is uncolored, but linked has color
        linked     myproj-feat-x  #4a8b5c
        main       ~/code/myproj  (no color)
        hint       set main color first: ccws --color '#4a8b5c' ~/code/myproj
  ok    workspace ready
        ~/code/myproj.code-workspace  #cc7700
```

랜덤 색이 적용된 결과 ok 배지가 따라옴. linked의 색을 그대로 인용해 사용자가 copy-paste로 main을 명시적으로 fix할 수 있게 한다.

`tui.Writer.Warn` 배지 + 기존 `Details` primitive 재사용. tui 패키지 변경 없음.

## 5. 패키지 구조

```
internal/
├── gitworktree/                ← 새 패키지
│   ├── gitworktree.go          (List, IdentityHash)
│   └── gitworktree_test.go
│   └── gitworktree_integration_test.go  (//go:build integration)
├── color/
│   ├── ladder.go               ← 새 파일 (LadderOffset, ApplyLightness)
│   └── ladder_test.go
└── runner/
    ├── resolve.go              ← 수정
    └── resolve_test.go         ← 확장
```

**DAG 변경:**

```
gitworktree → (stdlib only — os/exec, bufio, errors)
color       → (stdlib only)               ← 변화 없음
runner      → color, workspace, vscodesettings, gitworktree   ← gitworktree 추가
```

CLAUDE.md의 strict DAG 원칙 유지. `gitworktree`는 어떤 internal 패키지도 import하지 않는다 — anchor 색 파싱은 runner가 담당한다 (gitworktree가 워크스페이스 파일 경로를 반환하면 runner가 `vscodesettings`/`workspace`를 사용해 읽는다).

## 6. `internal/gitworktree` API

```go
package gitworktree

// Worktree describes a single git worktree as reported by `git worktree list --porcelain`.
type Worktree struct {
    Path   string  // absolute path to the working tree root
    GitDir string  // absolute path to .git dir or .git/worktrees/<name>
    Branch string  // empty if detached HEAD
    IsMain bool    // true for the primary worktree
}

// List runs `git worktree list --porcelain` from targetDir and returns all
// worktrees of the repo. The main worktree is first in the slice.
// Returns ErrNotInWorktree if targetDir is not under any git repo, or if
// git is not available. Other git failures are wrapped.
func List(targetDir string) ([]Worktree, error)

// FindSelf returns the worktree whose Path is targetDir (or the closest
// ancestor in worktrees), or nil if no match.
func FindSelf(worktrees []Worktree, targetDir string) *Worktree

// IdentityHash returns a stable 64-bit hash for a worktree.
//   - Main worktree → 0 (anchor offset = 0%).
//   - Linked → FNV-1a hash of basename(GitDir), i.e., the name git assigns
//     under .git/worktrees/<name>. Stable across branch and directory renames.
func IdentityHash(w Worktree) uint64

var ErrNotInWorktree = errors.New("gitworktree: target is not in a git worktree")
```

`List`는 git 명령 실패를 포함한 모든 에러 케이스에서 호출자가 Case "비-git/git 부재" 분기로 빠질 수 있도록, 명령이 한 번이라도 실패하면 일률적으로 `ErrNotInWorktree`를 반환한다. 진단 가능성을 위해 wrap한 원본 에러를 함께 보존한다 (`errors.Is(err, ErrNotInWorktree)`로 체크).

## 7. `internal/color/ladder.go` API

```go
package color

// LadderSteps are the lightness deltas (in HSL %) assigned by hash bucket.
// Six positions, symmetric around 0, ordered for stable bucket assignment.
// Excludes 0 — main worktree gets offset 0 by convention (IdentityHash returns 0).
var LadderSteps = []float64{-15, -10, -5, +5, +10, +15}

// LadderOffset returns the lightness delta (%) for a hash.
//   - hash == 0 → 0 (main).
//   - else → LadderSteps[hash % len(LadderSteps)].
func LadderOffset(hash uint64) float64

// ApplyLightness returns c with its HSL lightness shifted by deltaPct.
// L is clamped to [5, 95] to keep colors readable. deltaPct == 0 returns c.
func (c Color) ApplyLightness(deltaPct float64) Color
```

`Color`에는 이미 `Lighten`/`Darken` 메서드가 있고 HSL의 S/L에 대해 `clamp01`이 적용되어 있다 (`internal/color/primitives.go:48-49`). 따라서 `ApplyLightness`는 단순 위임:

```go
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

명시적인 [5, 95] clamp는 추가하지 않는다 — Lighten/Darken의 [0, 1] HSL clamp로 충분하고, "5% 이하의 거의 검정"은 워크트리 파생 색의 자연스러운 끝값이기 때문.

## 8. Runner 레이어 변경

**책임 분리:** `ResolveColor`는 색 결정만 하는 순수 함수로 유지하고 사이드 이펙트(main 워크스페이스 자동 작성)는 `runner.Run`이 담당한다. 워크트리 로직은 색 결정과 사이드 이펙트 의도를 함께 반환한다.

**`internal/runner/resolve.go` 변경:**

```go
type ColorSource int

const (
    SourceFlag     ColorSource = iota + 1
    SourceSettings
    SourceWorktree              // NEW
    SourceRandom
)

// AnchorIntent describes a side effect to write peacock keys into the main
// worktree's .code-workspace. Returned only in Case C (auto-establish).
type AnchorIntent struct {
    WorkspacePath string       // main worktree's <parent>/<dirname>.code-workspace
    AnchorColor   color.Color  // the random color chosen as family anchor
}

// ResolveColor returns the color, its source, optional info messages,
// optional anchor side-effect intent, and an error.
// The caller (runner.Run) is responsible for executing AnchorIntent if non-nil.
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, []string, *AnchorIntent, error)
```

내부 흐름:

```go
func ResolveColor(targetDir, flag string) (color.Color, ColorSource, []string, *AnchorIntent, error) {
    if flag != "" {
        c, err := color.Parse(flag)
        if err != nil { return ..., fmt.Errorf("--color: %w", err) }
        return c, SourceFlag, nil, nil, nil
    }

    c, src, warns, intent, ok, err := resolveFromWorktree(targetDir)
    if err != nil { return ..., err }
    if ok {
        return c, src, warns, intent, nil
    }
    // ok == false: 워크트리 로직이 색 결정에 실패. warns가 있을 수 있음 (Case D).
    accumulatedWarns := warns

    // 기존 흐름 (T의 settings.json → 랜덤)
    s, err := vscodesettings.Read(filepath.Join(targetDir, ".vscode", "settings.json"))
    if err != nil { return ..., err }
    if s != nil {
        if pc, ok := s.PeacockColor(); ok {
            c, err := color.Parse(pc)
            if err != nil { return ..., fmt.Errorf("peacock.color in settings: %w", err) }
            return c, SourceSettings, accumulatedWarns, nil, nil
        }
    }
    return color.Random(), SourceRandom, accumulatedWarns, nil, nil
}

// resolveFromWorktree returns:
//   (color, SourceWorktree, warnings, nil,         ok=true,  nil)  — Case A (main has color)
//   (color, SourceWorktree, warnings, anchorIntent, ok=true, nil)  — Case C (auto-establish)
//   (zero,  0,              warnings, nil,         ok=false, nil)  — Case B / D / non-worktree (warnings carry Case D notice)
//   (zero,  0,              nil,      nil,         false,    err)  — hard error (rare)
func resolveFromWorktree(targetDir string) (color.Color, ColorSource, []string, *AnchorIntent, bool, error)
```

`resolveFromWorktree` 내부 (의사 코드):

```go
worktrees, err := gitworktree.List(targetDir)
if errors.Is(err, gitworktree.ErrNotInWorktree) {
    return zero, 0, nil, nil, false, nil    // silent skip → fall through
}
if err != nil { return zero, 0, nil, nil, false, err }

self := gitworktree.FindSelf(worktrees, targetDir)
if self == nil { return zero, 0, nil, nil, false, nil }

main := worktrees[0]                                 // List 보장: main 첫번째
mainWsPath := workspaceFilePath(main.Path)
mainColor, err := readWorkspacePeacockColor(mainWsPath)   // 신규 헬퍼
if err != nil { return zero, 0, nil, nil, false, err }

// Case A
if mainColor != nil {
    offset := color.LadderOffset(gitworktree.IdentityHash(*self))
    return mainColor.ApplyLightness(offset), SourceWorktree, nil, nil, true, nil
}

linkedWithColor := findLinkedWithColor(worktrees, self)   // self 제외, 색 가진 첫 worktree

// Case D
if linkedWithColor != nil {
    warning := formatFamilyDisabledWarning(linkedWithColor, main)
    return zero, 0, []string{warning}, nil, false, nil
}

// Case B
if self.IsMain {
    return zero, 0, nil, nil, false, nil
}

// Case C: 자동 anchor 수립 (사이드 이펙트는 runner가 실행)
anchor := color.Random()
intent := &AnchorIntent{WorkspacePath: mainWsPath, AnchorColor: anchor}
offset := color.LadderOffset(gitworktree.IdentityHash(*self))
warning := formatAnchorCreatedWarning(mainWsPath, workspaceFilePath(self.Path))
return anchor.ApplyLightness(offset), SourceWorktree, []string{warning}, intent, true, nil
```

신규 헬퍼:
- `workspaceFilePath(workTree string) string` — `<parent>/<basename>.code-workspace` 계산. 이미 `runner.go`에 있음 (커밋 `a7e964c`)
- `readWorkspacePeacockColor(path string) (*color.Color, error)` — 워크스페이스 파일을 `workspace.Read`한 뒤 peacock.color를 추출. 파일 없음/peacock 키 없음은 `(nil, nil)`
- `findLinkedWithColor(worktrees []Worktree, exceptSelf *Worktree) *Worktree` — 각 linked의 `<parent>/<basename>.code-workspace`에서 peacock.color를 찾고 첫 매칭 반환

**`internal/runner/runner.go`의 Run에서 호출 부분:**

```go
c, src, resolveWarns, anchorIntent, err := ResolveColor(abs, opts.ColorInput)
if err != nil { return nil, err }
res.Warnings = append(res.Warnings, resolveWarns...)

// Case C 사이드 이펙트: main의 .code-workspace에 anchor 색 적용
if anchorIntent != nil {
    if err := writeAnchorWorkspace(anchorIntent, opts); err != nil {
        return nil, fmt.Errorf("write main anchor workspace: %w", err)
    }
}

// 기존 흐름 — settings.json read → Guard 2 → palette → Write → cleanup → Open
```

`writeAnchorWorkspace`는 main의 `.code-workspace`를 읽고 (없으면 새 Workspace), `workspace.EnsureFolder` + `workspace.ApplyPeacock(ws, anchor.Hex(), color.Palette(anchor, opts.Palette))`로 머지한 뒤 `workspace.Write`. main의 `.vscode/settings.json`은 절대 건드리지 않는다.

`Result`에 새 필드 추가하지 않는다 — `Warnings`에 메시지를 append하면 기존 렌더 파이프라인이 yellow warn 배지로 출력한다. ColorSource만 새 값을 추가한다.

## 9. CLI 렌더링 변경

`renderSuccess` / `renderWarnings` / `renderPreconfigured`는 그대로 유지. ColorSource label만 추가:

```go
// cmd/ccws/render.go
func sourceLabel(s runner.ColorSource) string {
    switch s {
    case runner.SourceFlag:     return "from --color flag"
    case runner.SourceSettings: return "from .vscode/settings.json"
    case runner.SourceWorktree: return "from worktree family"   // NEW
    case runner.SourceRandom:   return "random"
    }
    return ""
}
```

워크트리 로직이 추가한 warning은 `result.Warnings`에 들어있으므로 기존 `renderWarnings(tui.NewStderr(), res.Warnings)` 호출이 그대로 yellow warn 배지 + Details로 출력한다. 추가 렌더링 함수 불필요.

## 10. 인터랙티브 모드와의 상호작용

`ccws interactive`의 흐름은 변경 없음. Phase A pre-check (Guard 1) → form → 적용. 워크트리 로직은 색 결정 단계에서 발생하므로:

- form 진입 전: 기존과 동일
- form에서 사용자가 색을 명시적으로 입력하면 그것이 `--color` 동등하게 우선
- form에서 색을 비워두고 진행하면 worktree-aware ResolveColor가 호출됨 → Case A/B/C/D 분기

Case C가 인터랙티브에서 발생하는 경우(linked에서 시작, 색 비워둠) main의 `.code-workspace`가 자동 생성되며, warning이 기존 `renderWarnings` 경로로 stderr에 출력된다. form 흐름 중간에 추가 prompt 없음.

## 11. 테스트

**`internal/gitworktree/gitworktree_test.go`:**

- `TestList_ParsePorcelain` — `git worktree list --porcelain` 출력 fixture (string으로 inject 가능하도록 List 내부에 파서를 분리)
  - main + 2 linked
  - bare repo (`bare` 첫 줄 → `ErrNotInWorktree`)
  - detached HEAD (branch 빈 string)
  - branch만 있고 path 없는 비정상 케이스 (skip)
- `TestIdentityHash_MainReturnsZero`
- `TestIdentityHash_LinkedStable` — 같은 GitDir 두 번 호출 시 같은 결과
- `TestIdentityHash_DifferentLinked_DifferentHash` — 두 fixture worktree가 다른 hash (충돌 가능성은 있지만 fixture 기준)
- `TestFindSelf_ExactPath` / `_Subdir` / `_NoMatch`

**`internal/gitworktree/gitworktree_integration_test.go` (`//go:build integration`):**

- 실제 git 호출. 임시 dir에서 `git init`, 두 번 commit, `git worktree add` 후 `List` 호출 → 결과 검증
- git 바이너리 없으면 `t.Skip`

**`internal/color/ladder_test.go`:**

- `TestLadderOffset_Zero` → 0
- `TestLadderOffset_NonZero_InRange` — 임의 hash로 호출 시 결과가 LadderSteps 안에 있음
- `TestLadderOffset_Distribution` — 1000개 랜덤 hash → 각 버킷 100~250 사이 (chi-square 같은 정밀 분포 검증은 안 함, 균등성만 확인)
- `TestApplyLightness_Positive` — HSL L+5% (Lighten 위임 검증)
- `TestApplyLightness_Negative` — HSL L-5% (Darken 위임 검증)
- `TestApplyLightness_Zero` → 입력 그대로 (no-op)

**`internal/runner/resolve_test.go` 확장:**

`gitworktree.List`를 인터페이스로 추상화해 test에서 fake 주입:

```go
type worktreeLister interface {
    List(targetDir string) ([]gitworktree.Worktree, error)
}
```

기본 구현은 `gitworktree.List`. 테스트는 fake를 주입해 6개 케이스를 모두 커버:

- `TestResolveColor_FlagWins` — 기존 통과
- `TestResolveColor_SettingsWins_NotInWorktree` — fake가 `ErrNotInWorktree` → settings.json 경로
- `TestResolveColor_RandomFallback_NotInWorktree` — fake가 `ErrNotInWorktree` + settings 없음
- `TestResolveColor_WorktreeCaseA_MainHasColor_LinkedTarget` — main의 .code-workspace에 색, target은 linked → 오프셋 적용된 색, `SourceWorktree`
- `TestResolveColor_WorktreeCaseA_MainHasColor_MainTarget` — main의 색 그대로 (오프셋 0), `SourceWorktree`
- `TestResolveColor_WorktreeCaseB_MainTargetEmpty_FallthroughRandom`
- `TestResolveColor_WorktreeCaseC_LinkedFirst_AutoAnchor` — main 색 없음 + 다른 linked도 없음 + target은 linked → main의 .code-workspace 작성됨, target에 오프셋 적용된 색, warning에 anchor 메시지 포함
- `TestResolveColor_WorktreeCaseD_LinkedHasColor_MainEmpty_TargetMain` — warning에 family disabled 포함, fallback random
- `TestResolveColor_WorktreeCaseD_LinkedHasColor_MainEmpty_TargetOtherLinked` — 위와 동일 분기

**기존 `resolve_test.go` 4개 케이스 (`TestResolveColor_ExplicitWins`, `_InheritFromSettings`, `_Random`, `_InvalidFlag`):** `ResolveColor` 시그니처가 5-tuple로 바뀌므로 `got, src, _, _, err := ResolveColor(...)` 형태로 업데이트. 동작 검증은 그대로. 비-git 임시 디렉토리에서 실행되므로 worktree 로직은 자동 skip.

**`internal/runner/runner_test.go` 회귀:**

`runner.go:101`의 호출부도 새 시그니처에 맞춰야 함. runner.Run의 검증 테스트들은 fake lister 주입 또는 비-git 임시 디렉토리 사용으로 worktree 로직 영향 없음.

**Manual smoke (CLAUDE.md 패턴):**

- `git init` + `git commit` + `git worktree add` 후 ccws를 main, linked 순서로 실행 → linked 색이 main에서 ±5/±10/±15% 다른지 시각 확인
- 위를 linked, main 순서로 실행 → linked에서 첫 ccws 시 main의 `.code-workspace` 자동 생성 + warn 출력 확인
- linked에 미리 색 박아둔 뒤 main에서 ccws → family disabled warn 확인
- 비-git 디렉토리에서 ccws → 기존 흐름 유지 확인

## 12. 엣지 케이스

| 케이스 | 처리 |
|---|---|
| git 바이너리 없음 | `gitworktree.List` → `ErrNotInWorktree` → silent skip |
| target이 git repo 아님 | 동일 |
| bare repo | List가 첫 줄 `bare`를 감지해 `ErrNotInWorktree` 반환 → silent skip |
| detached HEAD | Branch 빈 string. IdentityHash는 GitDir basename 사용이므로 영향 없음 |
| `git worktree move` 후 ccws | git이 GitDir의 name 보존 → hash 안정 |
| `git worktree repair` 필요한 손상 상태 | `List`가 비정상 출력 → `ErrNotInWorktree` 반환 → silent skip |
| 6개 이상 linked worktree | LadderSteps 충돌 가능. 받아들임. anchor와는 항상 다름 (0%는 main 전용) |
| main의 `.code-workspace`가 다른 사용자 설정 갖고 있음 | `workspace.Merge`로 peacock 키만 머지, 사용자 설정 보존 |
| Case C에서 main의 `.code-workspace` 작성 실패 (권한 등) | hard error → ccws 실패. silent fallback 안 함 (anchor 없으면 family도 의미 없음) |
| target이 main의 `.git/worktrees/<name>` 같은 비정상 경로 | FindSelf가 nil 반환 → silent skip |

## 13. 마이그레이션 노트

**Behavior change:** prior versions used `--color > .vscode/settings.json > random` for color resolution. As of this version, when target is inside a git worktree, the worktree family logic is inserted between settings.json and random. Specifically:
- If the main worktree's `.code-workspace` already has a color, linked worktrees of the same repo automatically derive a related color (same hue/saturation, lightness shifted).
- If a linked worktree is the first to receive `ccws`, ccws will create the main worktree's `.code-workspace` with a random anchor color, then derive the linked worktree's color from it. A warn-level notice is emitted.
- Pass `--color` to bypass the worktree logic entirely.

기존 사용자 환경에 미치는 영향: 워크트리를 안 쓰는 사용자는 변화 없음. 워크트리를 쓰던 사용자는 두 번째 ccws부터 색이 family화됨 (이전 색이 random이었던 점은 변하지 않음).

`README.md`와 `CLAUDE.md` 양쪽에 이 노트를 넣는다.

## 14. 문서 변경

**`README.md`:**

- "Why" 또는 "Usage" 섹션 다음에 **"Worktree color family"** 짧은 섹션 추가:
  ```
  ## Worktree color family

  When you run ccws inside a git worktree, it automatically derives a "family"
  color from the main worktree's color. Linked worktrees get the same hue but
  shifted lightness, so they look related but distinct in your VSCode windows.

  - First ccws on main: random color, becomes the family anchor.
  - First ccws on a linked worktree (main not yet colored): a random anchor is
    written to the main worktree's `.code-workspace` automatically, and the
    linked worktree gets a derived color. A warning is printed.
  - If linked worktrees already have colors but main does not, ccws assumes
    the user set them deliberately and disables family logic with a warning.
  - Pass `--color` to bypass family logic.
  ```
- "Resolution priority" 또는 동등한 항목이 있다면 worktree 단계를 끼워 넣음.

**`CLAUDE.md`:**

- "Package import rule" DAG에 `gitworktree` 추가:
  ```
  gitworktree → (stdlib only)
  runner → color, workspace, vscodesettings, gitworktree
  ```
- "Non-goals" 섹션에 추가: "비-워크트리 환경에서 색 유사화", "사용자 정의 hash 시드".
- 색 결정 우선순위 언급이 있다면 worktree 단계 추가.

## 15. 변경 파일 요약

| 파일 | 동작 |
|---|---|
| `internal/gitworktree/gitworktree.go` | 신규. `Worktree`, `List`, `FindSelf`, `IdentityHash`, `ErrNotInWorktree` |
| `internal/gitworktree/gitworktree_test.go` | 신규. 파서/hash/FindSelf 단위 테스트 |
| `internal/gitworktree/gitworktree_integration_test.go` | 신규. `//go:build integration`. 실제 git 호출 |
| `internal/color/ladder.go` | 신규. `LadderSteps`, `LadderOffset`, `ApplyLightness` |
| `internal/color/ladder_test.go` | 신규 |
| `internal/runner/resolve.go` | `ResolveColor` 시그니처를 5-tuple `(color, source, warnings, anchorIntent, error)`로 확장, `AnchorIntent` 타입, `resolveFromWorktree`, `readWorkspacePeacockColor`, `findLinkedWithColor`, `formatFamilyDisabledWarning`, `formatAnchorCreatedWarning` 헬퍼 추가, `SourceWorktree` 추가 |
| `internal/runner/resolve_test.go` | 기존 4 테스트 시그니처 업데이트 + Case A/B/C/D 테스트 추가 |
| `internal/runner/runner.go` | `ResolveColor` 5-tuple 수신, `anchorIntent != nil`이면 `writeAnchorWorkspace` 호출 (main의 .code-workspace에 anchor palette 머지) |
| `cmd/ccws/render.go` | `sourceLabel`에 `SourceWorktree` 케이스 추가 |
| `Taskfile.yml` | (선택) 통합 테스트용 task 추가 (`task test:integration`) |
| `README.md` | §14 |
| `CLAUDE.md` | §14 |
