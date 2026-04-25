# CLI message styling — Design

## 1. 문제와 목표

현재 `ccws`의 모든 사용자 출력은 plain `fmt.Print*` 한 줄짜리이고, guard 에러는 메시지 본문에 `\n`과 comma-joined key 리스트가 baked-in 되어 있다. 결과적으로 17개 키가 충돌하면 한 줄로 길게 나오고, `--force` 힌트가 묻혀버려 가독성이 떨어진다.

목표: 모든 CLI 출력(success / warning / error / guard)을 일관된 badge + detail 레이아웃으로 정리하고, color/TTY 정책을 명확히 한다.

Non-goals: 인터랙티브 모드의 `huh` 자체 chrome 변경, log levels, structured logging(JSON), i18n.

## 2. 출력 인벤토리

`fmt.Print*`로 보내는 모든 메시지를 다음 4종류로 정리한다.

| 종류 | 현재 위치 | 새 표현 |
|---|---|---|
| 성공 | `cmd/ccws/root.go:56-57`, `cmd/ccws/interactive.go:47-48` | `ok` 배지 + detail rows (file, color) |
| 경고 | `cmd/ccws/root.go:58-59`, `cmd/ccws/interactive.go:49-50` (`Result.Warnings` 순회) | `warn` 배지 + 본문, 다중 경고는 빈 줄로 구분 |
| 일반 에러 | `cmd/ccws/main.go:13` (non-guard error) | `error` 배지 + 한 줄 본문 |
| Guard 에러 | 위 main.go 경로 (`*runner.GuardError`로 분기) | `error` 배지 + `file/keys/hint` detail 블록 |

Interactive 모드의 huh confirm dialog (`cmd/ccws/interactive.go:73-85`) 은 huh가 자체 border를 그리므로, 우리는 description 본문만 detail 형식으로 다시 만든다 (배지 없음).

## 3. 렌더링된 모양

**성공:**
```
  ok    wrote ~/Projects/foo.code-workspace
        color  #4a9d3c (from --color)
```

**경고:**
```
  warn  parent directory ~/Projects is a git repository;
        workspace file may be committed
```

**일반 에러:**
```
  error  target is not a directory: /tmp/foo.txt
```

**Guard 1 (예: 사용자가 보고한 17-키 케이스):**
```
  error  guard 1: existing peacock settings would be overwritten
         file  ~/Projects/vscode-color-workspace.code-workspace
         keys
           • settings.peacock.color
           • settings.workbench.colorCustomizations.activityBar.activeBackground
           • settings.workbench.colorCustomizations.activityBar.background
           • settings.workbench.colorCustomizations.activityBar.foreground
           • settings.workbench.colorCustomizations.activityBar.inactiveForeground
           • settings.workbench.colorCustomizations.activityBarBadge.background
           • settings.workbench.colorCustomizations.activityBarBadge.foreground
           • settings.workbench.colorCustomizations.commandCenter.border
           …(9 more)
         hint  rerun with --force to overwrite
```

**Guard 2:** 같은 모양, 타이틀만 `guard 2: non-peacock keys would remain in .vscode/settings.json`, keys는 잔여 키.

**Bullet 절단:** `maxBulletsShown = 8`. 8개 초과 시 `…(N more)`. 24-row 터미널에서 badge + hint 포함해도 스크롤 없이 보이는 선.

**Path shortening:** `$HOME` prefix면 `~/...`로 축약. 그 외 경로는 그대로.

## 4. 패키지 구조 — `internal/tui` + `cmd/ccws/render.go`

**경계:** `internal/tui`는 stdlib + `github.com/charmbracelet/lipgloss` + `github.com/mattn/go-isatty`만 import. 둘 다 `huh`를 통해 이미 transitive dep으로 들어와 있으므로 새 의존성 추가 없음. `tui`는 도메인 타입(`runner.GuardError` 등)을 모르고, 순수 presentation primitive만 제공한다.

**도메인 → tui 변환은 `cmd/ccws/render.go` 가 담당.** `errors.As`로 `*runner.GuardError`를 꺼내서 `tui`의 primitive를 조립. 이렇게 분리하면 `tui`는 라이브러리로서 깨끗하고, `runner`도 `tui`를 모른 채 유지.

**DAG (CLAUDE.md):**
- `tui` → (stdlib + lipgloss + isatty)
- `cmd/ccws` → `tui` + `runner` + `interactive` (기존)

`runner`나 `workspace`, `vscodesettings`는 `tui`를 import 하지 않는다.

**`internal/tui` API:**

```go
package tui

type Writer struct {
    out   io.Writer
    color bool
}

func NewStdout() *Writer                        // os.Stdout, isatty + NO_COLOR 자동 감지
func NewStderr() *Writer                        // os.Stderr, 동일 정책
func NewWriter(out io.Writer, color bool) *Writer // 테스트용 — bytes.Buffer 주입

// Badges — 한 줄짜리 라벨을 색 칠해진 cell로 prefix.
func (w *Writer) OK(title string)
func (w *Writer) Warn(title string)
func (w *Writer) Error(title string)

// Detail rows — 라벨 컬럼 정렬, 배지 컬럼 아래에 indent.
type Detail struct{ Label, Value string }
func (w *Writer) Details(rows []Detail)

// Bullet list — keys 같은 가변 길이 리스트. max 초과 시 "…(N more)".
func (w *Writer) Bullets(items []string, max int)

// Path 축약 헬퍼 (테스트 용이성을 위해 export).
func ShortenPath(p string) string  // $HOME prefix면 ~/..., 아니면 그대로
```

**`cmd/ccws/render.go` 책임:**

```go
// renderError dispatches by error type. Called once from main.go.
func renderError(w *tui.Writer, err error) {
    var ge *runner.GuardError
    if errors.As(err, &ge) {
        renderGuard(w, ge)
        return
    }
    w.Error(err.Error())
}

// renderGuard composes badge + details + bullets + hint.
func renderGuard(w *tui.Writer, ge *runner.GuardError) { ... }

// guardDescription returns plain (no-badge) text for the huh confirm body.
func guardDescription(ge *runner.GuardError) string { ... }

// renderSuccess / renderWarnings — used by root.go & interactive.go.
func renderSuccess(w *tui.Writer, res *runner.Result, srcLabel string) { ... }
func renderWarnings(w *tui.Writer, warnings []string) { ... }
```

## 5. GuardError 데이터화

현재:
```go
type GuardError struct {
    Guard   int
    Message string   // 포맷 문자열이 baked-in
    Keys    []string
}
```

변경:
```go
type GuardError struct {
    Guard int
    Path  string     // Guard 1: workspace file, Guard 2: settings.json
    Keys  []string
}

func (e *GuardError) Error() string {
    return fmt.Sprintf("guard %d: %d conflicting keys in %s", e.Guard, len(e.Keys), e.Path)
}
```

- `Message` 필드 제거. 임베디드 `\nrerun with --force…` 힌트 제거.
- `Error()`는 newline 없는 한 줄 — `%v`, log line, `errors.Is/As` fallback에 안전.
- 화려한 출력은 `tui.RenderError`만 책임짐.

**Breaking change scope:** `runner` 패키지는 외부 consumer가 없음 (이 repo의 `cmd/ccws` + 내부 테스트만). `runner_test.go`의 GuardError 검증을 새 필드 형태로 업데이트.

## 6. main.go 진입점

```go
func main() {
    cmd := rootCmd()
    cmd.SilenceErrors = true
    cmd.SilenceUsage = true
    if err := cmd.Execute(); err != nil {
        renderError(tui.NewStderr(), err)
        os.Exit(errToExit(err))
    }
}
```

`renderError` 분기 (in `cmd/ccws/render.go`):
- `*runner.GuardError` → §3의 full block (`Error` 배지 + `Details` + `Bullets` + hint)
- 그 외 → `w.Error(err.Error())` (한 줄)

성공/경고 출력은 root.go / interactive.go에서 `renderSuccess` / `renderWarnings` 호출 — 둘 다 `tui.NewStdout()` / `tui.NewStderr()` 인스턴스를 받는다.

## 7. Color / TTY 정책

`NewStdout`/`NewStderr` 생성 시:
```
color = isatty(fd) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
```

- `NO_COLOR`: lipgloss도 자체적으로 honor 하지만, 명시적으로 게이트해서 layout(배지 padding 셀)도 같이 collapse 시킨다.
- 비-TTY일 때: ANSI 없음, padded badge cell 없음. 그냥 `error: ...`, `warn: ...`, `ok: ...` plain prefix. `ccws | grep`이나 `ccws 2>err.log`가 깨끗하게 나옴.
- `mattn/go-isatty`는 huh가 transitive로 가져옴. 직접 import.

## 8. Palette (lipgloss ANSI 16색)

| 요소 | bg | fg | 기타 |
|---|---|---|---|
| `error` 배지 | `9` (red) | `15` (white) | bold, lipgloss `Padding(0, 1)` (좌우 1칸) |
| `warn` 배지 | `11` (yellow) | `0` (black) | bold, `Padding(0, 1)` |
| `ok` 배지 | `10` (green) | `0` (black) | bold, `Padding(0, 1)` |
| Detail 라벨 (`file`, `keys`, `hint`) | — | `8` (dim) | — |
| `hint` value | — | `6` (cyan) | — |
| Bullet glyph `•` | — | `8` (dim) | — |
| Bullet value | — | default | — |

Truecolor 안 씀 — 16색이면 CI 로그/ssh/저색 터미널에서도 안전. lipgloss가 자동으로 degrade.

## 9. 테스트

**`internal/tui/tui_test.go` (신규):**
- 모든 케이스 `color=false`로 deterministic snapshot 비교.
- `OK + Details`, `Warn`, `Error` 단일 라인.
- `Error + Details + Bullets`: 5 키 (절단 안 됨) / 17 키 (8 + `…(9 more)`) / 0 키 edge case.
- `ShortenPath`: `$HOME=/tmp/h`, path=`/tmp/h/x` → `~/x`. Non-prefix는 그대로. `$HOME` unset도 처리.
- 한 개의 `color=true` smoke test: 출력에 `\x1b[`가 포함되는지 확인 (특정 코드 pin 안 함).

**`cmd/ccws/render_test.go` (신규, optional):**
- `renderError(*GuardError)`와 `renderError(plain error)`의 출력 비교.
- `*tui.Writer`를 `bytes.Buffer`로 주입할 수 있도록 `tui`에 작은 생성자 헬퍼 추가 (e.g., `NewWriter(io.Writer, color bool)`).

**`internal/runner/runner_test.go` (수정):**
- `GuardError.Message` 검증 → `Path` + `Keys` 검증으로 변경.
- `Guard1`, `Guard2` 두 테스트의 assertion 갱신.

**Manual 검증:**
- `task build && ./ccws .` (성공 케이스에서 ok 배지 색)
- 사용자가 보고한 케이스 재현 — 17-키 .code-workspace에 대해 guard 1 색 출력 확인.
- `./ccws . | cat` (pipe → ANSI/badge 없음)
- `NO_COLOR=1 ./ccws .` (env → 컬러 없음)

## 10. 변경 파일 요약

| 파일 | 동작 |
|---|---|
| `internal/tui/tui.go` | 신규 — Writer, badges, Details, Bullets, ShortenPath |
| `internal/tui/tui_test.go` | 신규 — §9 |
| `cmd/ccws/render.go` | 신규 — renderError, renderGuard, guardDescription, renderSuccess, renderWarnings |
| `internal/runner/runner.go` | `GuardError` 필드 변경 (`Message` 제거, `Path` 추가), `Error()` 한줄 형식 |
| `internal/runner/runner_test.go` | GuardError assertion 업데이트 |
| `cmd/ccws/main.go` | err 출력 → `renderError(tui.NewStderr(), err)` |
| `cmd/ccws/root.go` | 성공/경고 출력 → `renderSuccess` / `renderWarnings` 호출 |
| `cmd/ccws/interactive.go` | 성공/경고 출력 → `renderSuccess` / `renderWarnings` 호출, `confirmGuard`는 `guardDescription(ge)`로 description 본문 작성 |
| `go.mod` / `go.sum` | `lipgloss`, `go-isatty`를 indirect → direct로 승격 |
| `CLAUDE.md` | DAG 섹션에 `tui → (stdlib + lipgloss + isatty)` 추가, render.go 위치는 cmd/ccws 안이라 DAG 변경 없음 |
