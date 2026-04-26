# Default to launch on existing peacock workspace — Design

## 1. 문제와 목표

오늘 `ccws`는 `<parent>/<folder>.code-workspace`에 이미 peacock 키가 들어있을 경우 Guard 1 에러를 내고 exit 2로 끝난다 (`internal/runner/runner.go:81-85`). 사용자 입장에서 흔한 시나리오 — "이 폴더는 예전에 이미 ccws로 색칠했고, 지금은 그냥 VSCode로 열고 싶다" — 가 매번 `--force` 또는 손수 `code <wsfile>`을 강요한다.

목표: 워크스페이스에 peacock 키가 이미 있으면 **덮어쓰지 않고**, 경고를 stderr로 띄우고, 그 워크스페이스를 그대로 `code`로 연 뒤 exit 0으로 종료하는 것을 새 기본 동작으로 만든다. 명시적으로 덮어쓰고 싶은 경우는 기존의 `--force`를 그대로 사용한다.

이 변경은 `2026-04-25-ccws-vscode-color-workspace-design.md`의 §safety-guards / §exit-codes 중 Guard 1 관련 부분을 supersede 한다. Guard 2 동작은 변경 없음.

Non-goals: 새 CLI 서브커맨드 추가, peacock 키 자동 갱신, 워크스페이스 파일 부분 머지 정책 변경, `.vscode/settings.json` 잔여 키 정리 정책 변경.

## 2. 트리거와 새 동작 매트릭스

"peacock 키가 있다"의 정의는 오늘과 동일: `workspace.ExistingPeacockKeys(ws)`가 비어있지 않음 (`internal/workspace/guard.go`). 즉 `peacock.*` 또는 `workbench.colorCustomizations` 안의 peacock-managed 키 중 하나라도 존재하면 트리거.

| 케이스 | 오늘 | 새 동작 |
|---|---|---|
| ws 파일 없음 | 새로 생성 + 색칠 + cleanup + open | 동일 |
| ws 파일 있음, peacock 키 **없음** | 머지로 peacock 키 추가 + cleanup + open | 동일 |
| ws 파일 있음, peacock 키 **있음**, `--force` 없음 | Guard 1 → exit 2 | 경고 stderr + `code <wsfile>` + **exit 0**. settings.json은 손대지 않음 (Guard 2 검사 자체를 건너뜀) |
| ws 파일 있음, peacock 키 **있음**, `--force` | 덮어쓰기 + cleanup + open | 동일 |
| Guard 2 (settings.json 잔여 색상 키) | 독립 검사 → exit 2 | 워크스페이스에 peacock 키 있으면 Guard 2 검사 미진입. 그 외 케이스는 동일 |
| `interactive` 진입 시 ws에 peacock 키 있음 | 폼 후 Guard 1 confirm → Yes면 force, No면 abort exit 2 | **폼 진입 전** 3-옵션 prompt: Open existing / Overwrite / Cancel |

`.vscode/settings.json`을 굳이 손대지 않는 이유: 이미 워크스페이스에 peacock 키가 자리잡았다는 것은 사용자가 한 번 ccws를 돌렸거나 수동으로 그 상태를 만들었다는 뜻이고, settings.json에 남아있는 색상 키도 사용자가 그렇게 두기로 한 결정일 수 있다. 새 기본 동작은 "이미 설정된 폴더에는 손대지 않는다"가 일관된 멘탈 모델이다. 명시적으로 정리하고 싶다면 `--force`로 전체 흐름을 다시 돌리면 된다.

## 3. 렌더링된 모양

**Preconfigured (새 기본 동작):**
```
  warn  workspace already configured
        workspace     ~/Projects/foo.code-workspace
        peacock keys  3 existing
        hint          use --force to overwrite (other flags ignored)
```

stderr로 출력. `tui.Writer.Warn` 배지 + 기존 `Details` primitive 재사용. peacock 키 카운트만 노출하고 키 이름 자체는 안 보여줌 (Guard 1 에러일 때처럼 길게 bullet으로 풀 필요 없음 — soft notice이므로).

배지/포맷이 OFF인 환경(NO_COLOR, 비-TTY)에서는 기존 정책대로 plain prefix만 출력.

## 4. Runner 레이어 변경

**`internal/runner/runner.go` Result 확장:**

```go
type Result struct {
    WorkspaceFile   string
    ColorHex        string       // empty when Preconfigured
    ColorSource     ColorSource  // zero when Preconfigured
    SettingsCleaned bool         // false when Preconfigured
    Preconfigured   bool         // NEW
    PeacockKeys     []string     // NEW: detected keys (rendering uses len only)
    Warnings        []string
}
```

**`Run` 흐름 변경 (간략):**

```
stat target → abs → wsPath
ws := workspace.Read(wsPath)

if ws != nil && !opts.Force {
    keys := workspace.ExistingPeacockKeys(ws)
    if len(keys) > 0 {
        result := &Result{WorkspaceFile: wsPath, Preconfigured: true, PeacockKeys: keys}
        if !opts.NoOpen {
            if err := r.Opener.Open(wsPath); err != nil {
                // ErrCodeNotFound → "code CLI not on PATH; open manually: <path>"
                // 그 외 → "failed to open with code: <err>"
                result.Warnings = append(result.Warnings, ...)
            }
        }
        return result, nil
    }
}

// 기존 흐름 (color resolve → Guard 2 → write → cleanup → open)
```

`*GuardError{Guard: 1}` 반환 경로는 사라진다. `GuardError` 타입 자체는 Guard 2를 위해 그대로 유지 (필드는 손 안 댐).

**왜 Result에 `Preconfigured` 플래그를 넣고 새 sentinel 에러를 안 만드는가:** Runner의 책임은 "워크스페이스를 만들/재사용/거부 중 하나의 결정을 내리는 것"이다. Preconfigured는 거부가 아니라 "재사용으로 결정함"이므로 success path에 속한다. Result에 두 가지 성공 모양을 명시적으로 구분하는 것이 의미상 맞다. Guard 2는 여전히 진짜 에러(사용자 개입 필요)이므로 `*GuardError`로 남는다.

## 5. CLI 렌더링 변경

**`cmd/ccws/render.go`에 `renderPreconfigured(w *tui.Writer, res *runner.Result)` 추가.** 기존 `renderSuccess` / `renderWarnings` / `renderError`와 같은 톤. `tui` 패키지에는 변경 없음 — 기존 Warn 배지 + Details primitive로 충분.

**`cmd/ccws/root.go` RunE 변경:**

```go
res, err := runner.New(nil).Run(opts)
if err != nil { return err }
if res.Preconfigured {
    renderPreconfigured(tui.NewStderr(), res)
} else {
    renderSuccess(tui.NewStdout(), res, sourceLabel(res.ColorSource))
}
renderWarnings(tui.NewStderr(), res.Warnings)
return nil
```

**Exit code (`errToExit`):** 변경 없음. Preconfigured는 `err == nil`이라 자동으로 0. Guard 2는 여전히 2. 기존 mapping 유지.

**플래그 무시 안내:** Preconfigured short-circuit 시 `--color`, `--keep-source` 등 워크스페이스 변경에 영향을 주는 다른 플래그는 자동으로 무효가 된다. Runner 안에서 별도 분기를 만들지 않고, `renderPreconfigured`의 hint 라인 ("other flags ignored") 한 줄로 통일해 안내한다. `--no-open`만은 Open() 호출 자체를 게이트하므로 의미 있게 동작한다.

## 6. Interactive 모드

**`cmd/ccws/interactive.go` 흐름 변경 — Phase A 사전 검사 도입:**

```
abs := target dir (CLI arg)
wsPath := <parent>/<folder>.code-workspace
ws := workspace.Read(wsPath)
keys := workspace.ExistingPeacockKeys(ws)

if len(keys) > 0:
    // Phase A: 3-옵션 select
    var choice string
    huh.NewSelect[string]().
        Title("Workspace already configured").
        Description(fmt.Sprintf("%s\n%d peacock keys present", tui.ShortenPath(wsPath), len(keys))).
        Options(
            huh.NewOption("Open existing workspace", "open"),
            huh.NewOption("Overwrite (start fresh)", "overwrite"),
            huh.NewOption("Cancel", "cancel"),
        ).Value(&choice).Run()

    switch choice {
    case "open":
        // Runner의 short-circuit 패스로 그대로 흘려보냄 (Force=false → Preconfigured 결과)
        opts := runner.Defaults()
        opts.TargetDir = abs
        res, err := runner.New(nil).Run(opts)
        if err != nil { return err }
        renderPreconfigured(tui.NewStderr(), res)
        renderWarnings(tui.NewStderr(), res.Warnings)
        return nil  // exit 0
    case "cancel":
        return nil  // exit 0
    case "overwrite":
        forcePreselected = true
        // fall through to Phase B
    }

// Phase B: 기존 huh 폼 흐름 (변경 없음)
choices, _ := interactive.Run(abs)
opts := interactive.ApplyToOptions(*choices, choices.TargetDir)
opts.Force = forcePreselected || opts.Force
// ... 기존 2-attempt loop (Guard 2 confirm용)
```

**엣지 케이스 — 사용자가 폼에서 target dir을 다른 preconfigured 폴더로 변경:** Runner.Run이 그 시점에 다시 Preconfigured=true를 반환하므로, 후처리에서 `res.Preconfigured`가 true면 `renderPreconfigured`로 출력하고 종료. 추가 prompt 없음. 즉 `for attempt := 0; attempt < 2; attempt++` 루프 안에서 `err == nil && res.Preconfigured`도 정상 종료 케이스로 같이 처리.

**Phase A 함수 분리:** 폼 없이 단위 테스트할 수 있도록 사전 검사 자체를 작은 함수로 빼냄:

```go
// detectPreconfigured returns the workspace path and existing peacock keys
// if the target's .code-workspace already has peacock keys; otherwise
// returns ("", nil, nil).
func detectPreconfigured(absTarget string) (string, []string, error)
```

3-옵션 select은 huh 의존이라 manual 검증 영역. `detectPreconfigured`만 unit test.

**Cancel exit code:** 사용자가 명시적으로 Cancel을 골랐으므로 exit 0. (오늘 Guard abort 시 exit 2와 다른 흐름 — Guard abort는 "동작이 막힘"이지만 새 Cancel은 "사용자가 액션 안 함을 선택").

## 7. 테스트

**`internal/runner/runner_test.go` 추가/수정:**

- `TestRun_Preconfigured_PeacockKeysPresent` — peacock 키 있는 ws 파일을 fixture로 두고, `Force=false`로 실행. 검증: `Result.Preconfigured == true`, `Result.WorkspaceFile == wsPath`, `Result.PeacockKeys` 비어있지 않음, ws 파일 mtime 변경 없음, `.vscode/settings.json` 손 안 댐 (read조차 안 일어나는지는 stub Opener/filesystem hook이 있어야 strict 검증 가능; 여기서는 파일 mtime 검증으로 대체).
- `TestRun_Preconfigured_NoOpen` — 위 + `NoOpen=true`. Opener stub의 호출 카운트 0 검증.
- `TestRun_Preconfigured_OpenerError` — Opener stub이 `ErrCodeNotFound` 반환. `Result.Warnings`에 "code CLI not on PATH" 포함, `Result.Preconfigured == true` 그대로.
- `TestRun_Force_OverridesPreconfigured` — peacock 키 있는 ws + `Force=true`. 오늘 흐름대로 overwrite, `Result.Preconfigured == false`. (기존 force-overwrite 테스트가 이미 커버하면 추가 안 함; 갱신만)
- `TestRun_NoPeacockKeys_MergeStillWorks` — 회귀 방지. ws 파일은 있지만 peacock 키 없음 → 머지로 peacock 추가, `Preconfigured == false`.
- `TestRun_Guard2_StillFires` — ws에 peacock 키 없음 + settings.json에 비-peacock 색상 키 → `*GuardError{Guard: 2}` 반환. 기존 테스트가 이미 있으면 그대로 통과해야 함.

`*GuardError{Guard: 1}` 반환을 검증하던 기존 테스트가 있다면 새 `Preconfigured` 검증으로 변환.

**`cmd/ccws/render_test.go` 추가:**

- `TestRenderPreconfigured_PlainOutput` — `tui.NewWriter(&buf, false)`로 색 OFF 렌더 후 substring 검증: `workspace already configured`, ShortenPath 적용된 경로, `3 existing`, `--force`, `other flags ignored`.
- `TestRenderPreconfigured_NoColorEnv` — `NO_COLOR` env 시 ANSI escape가 없는지.

**`cmd/ccws/interactive_test.go` 추가:**

- `TestDetectPreconfigured_PeacockKeysPresent` / `_NoKeys` / `_NoFile` — 임시 dir에 fixture .code-workspace 두고 함수 직접 호출.

**Manual smoke:**

- `task build && ./ccws .`을 두 번 호출. 첫 번째: ok 배지 + 새 색. 두 번째: warn 배지 + "already configured" + VSCode 다시 뜸.
- 위 두 번째 + `--force`: ok 배지, 새 색으로 덮어쓰기 됨.
- `./ccws --no-open .` 두 번째 호출: warn 배지만, VSCode 안 뜸.
- `./ccws interactive .` 사전 설정된 폴더에서: 3-옵션 select 등장, 각 옵션 동작 확인.
- `./ccws . | cat`: ANSI 없음. `NO_COLOR=1 ./ccws .`: 컬러 없음.

## 8. 마이그레이션 노트

**Behavior change:** prior versions exited with code 2 when the target's workspace file already contained peacock keys. As of this version, ccws prints a warning, opens the existing workspace, and exits 0. Any shell script that depended on the exit-2 path for this case must now check stderr for the "workspace already configured" notice or always pass `--force`.

`README.md`와 `CLAUDE.md` 양쪽에 이 노트를 넣는다.

## 9. 문서 변경

**`README.md`:**

- "Running `ccws` in `/home/me/code/myproj`..." 단계 3 갱신: "Write `<parent>/<folder>.code-workspace` (merging peacock keys into any existing file). **If the file already contains peacock keys, ccws skips the write and just opens it. Pass `--force` to overwrite.**"
- "Safety guards" 섹션의 Guard 1 항목을 soft notice로 재서술. Guard 2는 그대로.
- "Exit codes" 표 아래 footnote: "Code 2 covers Guard 2 only. The previous Guard-1 case (existing peacock keys in the workspace file) now exits 0 with a warning."

**`CLAUDE.md`:**

- "Safety guards" 섹션 갱신:
  ```
  - **Guard 1 (soft)** — existing Peacock keys in the target `.code-workspace`. Default: warn + open existing, exit 0. `--force` overwrites. When triggered, Guard 2 check is skipped too.
  - **Guard 2** — non-Peacock color keys would remain in `.vscode/settings.json` after cleanup. Default: exit 2. `--force` bypasses.
  - Exit-code mapping lives in `cmd/ccws/root.go:errToExit`. Interactive mode converts Guard 2 errors into huh confirms; Guard 1 (soft) is handled in Phase A pre-check before the form.
  ```
- "Exit codes" 매핑 줄 수정: `2`의 의미가 "safety guard triggered" → "Guard 2 triggered"로 좁아짐 (Guard 1은 더이상 exit 2가 아님).

**`docs/superpowers/specs/2026-04-25-ccws-vscode-color-workspace-design.md`:** 변경 없음. 본 spec이 supersede 표시는 본 문서 §1에 이미 적힘.

## 10. 변경 파일 요약

| 파일 | 동작 |
|---|---|
| `internal/runner/runner.go` | `Result`에 `Preconfigured`, `PeacockKeys` 필드 추가. ws 존재 + peacock 키 있고 `!Force`이면 short-circuit 분기 추가 |
| `internal/runner/options.go` | 변경 없음 |
| `internal/runner/runner_test.go` | §7의 테스트 추가/갱신 |
| `cmd/ccws/render.go` | `renderPreconfigured(*tui.Writer, *runner.Result)` 추가 |
| `cmd/ccws/render_test.go` | §7의 렌더 테스트 추가 |
| `cmd/ccws/root.go` | RunE에서 `res.Preconfigured` 분기 |
| `cmd/ccws/interactive.go` | Phase A 사전 검사 + 3-옵션 select 도입. 기존 폼 흐름은 Phase B로 유지. `detectPreconfigured` 헬퍼 추출 |
| `cmd/ccws/interactive_test.go` | `detectPreconfigured` unit test 추가 |
| `README.md` | §9의 문서 변경 |
| `CLAUDE.md` | §9의 문서 변경 |
