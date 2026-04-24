# vscode-color-workspace (`ccws`) — Design

## 1. 문제와 목표

Peacock VSCode extension은 프로젝트별 색 구분을 위해 `workbench.colorCustomizations`와 `peacock.*` 설정을 대상 폴더의 `.vscode/settings.json`에 저장한다. 이 파일은 Git으로 공유되는 경로라, 개인 구분용 색이 팀 레포에 섞여 들어가는 구조적 부조화가 발생한다 (상세 배경: `peacock.md`).

`ccws`는 Peacock을 완전히 대체한다. 목표:

- `.vscode/settings.json`을 건들지 않고 **폴더 상위의 `.code-workspace` 파일**에 색 설정을 쓴다.
- Peacock의 색 알고리즘(`prepareColors`)을 Go로 포팅해 동일한 결과를 낸다.
- `.vscode/settings.json`에 기존 Peacock 설정이 있으면 migration으로 옮기고 정리한다.
- 생성 후 `code` 명령으로 workspace 파일을 바로 연다.

Non-goals (v1 제외): Peacock favorites, remoteColor/Live Share, multi-root workspaces, element-별 adjustments, VSCode Profiles 통합, uninstall subcommand, `.code-workspace` 주석 보존.

## 2. CLI 표면

**서브커맨드 2개:**

```
ccws [<target-dir>] [flags]          # zero-config, 기본값으로 즉시 실행
ccws interactive [<target-dir>]      # huh form으로 옵션 선택
```

`<target-dir>` 생략 시 `$PWD`.

**플래그 (최소 3개):**

| 플래그 | 기본 | 용도 |
|---|---|---|
| `--color <hex\|name\|random>` | auto | `#RRGGBB`, `#RGB`, CSS 명명 색(`red` 등), 또는 `random`. 없으면 `.vscode/settings.json`의 `peacock.color` 승계 → 없으면 random |
| `--no-open` | off | 생성 후 `code`로 열지 않음 (CI/스크립트용) |
| `--force` | off | 두 safety guard(§6) 모두 bypass |

모든 `affect-*` / standard setting 노브는 flag에서 제거 — interactive 모드에서만 선택. 기본값은 Peacock과 동일.

## 3. 색 결정

우선순위 (위에서 아래):

1. `--color X` 명시됨 → X 사용 (항상 이김)
2. `.vscode/settings.json`에 `peacock.color` 존재 → 그 값
3. random (`math/rand.Float64()` × 3 R/G/B, Peacock의 `tinycolor.random()` 동작 매칭)

어느 쪽이든 **결정된 base 색 하나로 palette를 재생성**. 기존 `workbench.colorCustomizations` 맵을 verbatim 복사하지 않음 — 색이 single source of truth.

파싱: `github.com/lucasb-eyer/go-colorful`의 `Hex()` + CSS 명명색 lookup table + `#RGB` 3자리 확장.

## 4. Palette 생성 (Peacock 알고리즘 포팅)

**포팅 대상**: `/Users/user/Projects/vscode-peacock/src/color-library.ts` + `configuration/read-configuration.ts`의 `prepareColors` 및 5개 `collect*Settings` 함수.

**Primitives** (`internal/color/primitives.go`):

| Peacock (tinycolor2) | Go |
|---|---|
| 입력 파싱 | `colorful.Hex()` + 명명색 table + `#RGB` 확장 |
| `.toHexString()` | `fmt.Sprintf("#%02x%02x%02x", r, g, b)` |
| `.setAlpha(0x99/0xff).toHex8String()` | 위에 `%02x`(0x99) 추가 |
| `.isLight()` | tinycolor 기준 brightness ≥ 128 (`(r*299 + g*587 + b*114)/1000`) |
| `.lighten(n)` / `.darken(n)` | HSL → L ± n/100 → clamp [0,1] → RGB |
| `.triad()[1]` | HSL → H = (H + 120) mod 360 |
| `.complement()` | HSL → H = (H + 180) mod 360 |
| `readability(c1, c2)` | WCAG: `(L1+0.05)/(L2+0.05)`, 큰 쪽이 분자. 상대 luminance는 `0.2126*R + 0.7152*G + 0.0722*B` (linearized: ≤0.03928 → /12.92, else → ((+0.055)/1.055)^2.4) |

**`getReadableAccentColorHex(bg, ratio)`**: triad[1] 계산 → HSL 추출 → 회색이면 hue = `60 * round(L*6)` 재할당 → s<0.15이면 s=0.5 → L을 16단계로 샘플링해 각 단계 contrast ratio 계산 → contrast 기준 오름차순 정렬 → ratio 넘는 첫 색 채택 (없으면 `#ffffff`). Peacock 166줄 1:1 포팅.

**`getRandomColorHex()`**: tinycolor가 R/G/B 각각 `Math.random()`으로 뽑는 방식 그대로. `math/rand.Float64()` × 3. 시드: `time.Now().UnixNano()`.

**`Palette(base, opts) → map[string]string`** (`internal/color/palette.go`): `prepareColors` 포팅. `opts.Affect` (10개 bool), `opts.Standard` (keepForegroundColor, keepBadgeColor, squigglyBeGone, darkenLightenPct). 활성화된 affect에 해당하는 키만 출력.

`darkForegroundColor` / `lightForegroundColor`는 v1에선 Peacock 상수(`#15202b`, `#e7e7e7`) 하드코드, 노출 안 함.

**포팅 검증**: `/Users/user/Projects/vscode-peacock/src/test/`에 snapshot이 있으면 가져옴. 없으면 mini Node 스크립트로 `prepareColors`를 고정 base 5개(`#ff0000`, `#42b883`, `#5a3b8c`, `#000000`, `#ffffff`)에 실행, 결과를 JSON fixture로 덤프, Go 테스트에 `//go:embed`로 로드해 diff.

## 5. 파일 배치

**기본**: `<parent>/<target-folder-name>.code-workspace`.

예: target = `/Users/foo/Projects/asr-tts` → workspace 파일 = `/Users/foo/Projects/asr-tts.code-workspace`.

**`.code-workspace` 내용 기본 골격**:

```json
{
  "folders": [{ "path": "./<folder-name>" }],
  "settings": {
    "peacock.color": "#RRGGBB",
    "workbench.colorCustomizations": { ... 28-key 중 활성화된 것 ... }
  }
}
```

`folders[0].path`는 workspace 파일 기준 **상대 경로**. parent 디렉터리와 함께 이동해도 깨지지 않음.

**Override**: 없음(v1). 파일 경로 강제 지정 플래그는 interactive에서도 제공하지 않음. 필요하면 v2.

**Parent 쓰기 불가능**: exit 3.

**Parent가 git repo인 경우**: stderr 경고 — "parent directory is a git repository; workspace file may be committed. 고려하세요." 실행은 계속.

## 6. Safety guards

두 guard 모두 **read-only 체크, 어떤 파일도 건들지 않음**. 하나라도 트리거되면 exit 2, 어떤 키/어떤 파일인지 stderr 나열.

**Guard 1 — Workspace merge**:

기존 `.code-workspace`에서 `settings.workbench.colorCustomizations`의 키 중 ColorSettings enum에 속한 것이 하나라도 있거나, `settings.peacock.color` 또는 `settings["peacock.*"]`가 존재하면 트리거.

**Guard 2 — Source residual**:

Source cleanup(§7)이 수행될 때만 체크. `.vscode/settings.json`의 `workbench.colorCustomizations`에서 ColorSettings enum **외** 키가 하나라도 있으면 트리거 (Peacock 키 삭제 후 남는 비-peacock 색 설정). Tool의 의도가 "색 관련 설정을 `.vscode/settings.json`에서 싹 빼내는 것"이므로 남는 게 있으면 사용자가 수동으로 결정하도록 멈춤.

Interactive에서 "Delete peacock settings?" No를 선택한 경우 cleanup을 스킵하므로 Guard 2 불활성.

**Bypass**:
- Flag mode: `--force`로 둘 다 무시. Guard 1은 기존 peacock 키 overwrite, Guard 2는 비-peacock 키 그대로 두고 진행.
- Interactive: 해당 guard 트리거 시 flow 안에서 명시 Confirm 단계 (기본 No → abort).

## 7. Source cleanup (`.vscode/settings.json` 정리)

**대상** (정밀 삭제):
- `peacock.*` 키 전체 (`peacock.color`, `peacock.affectActivityBar` 등)
- `workbench.colorCustomizations` 내 ColorSettings enum 28개 키만

**후처리**:
- `workbench.colorCustomizations`가 `{}`가 되면 그 키 자체 삭제
- `.vscode/settings.json`이 `{}`가 되면 파일 삭제
- `.vscode/` 디렉터리가 빈 경우 디렉터리 삭제

**Interactive 옵션**: "Delete Peacock settings from .vscode/settings.json?" Confirm. No면 cleanup 스킵. Flag 모드에선 항상 delete (Guard 2가 이미 안전성 담보).

## 8. Interactive flow (`huh` form)

```
[Input]        Target directory (default: $PWD)
[Pre-check]    Safety guard가 트리거되면 해당 Confirm 삽입 (§6)

[Select]       Color source
                - Use existing peacock.color from .vscode/settings.json  (있을 때만)
                - Random
                - Custom
               └ (Custom 선택 시) [Input] Hex or CSS name

[MultiSelect]  Affected elements
                [x] activityBar       [ ] editorGroupBorder
                [x] statusBar         [ ] panelBorder
                [x] titleBar          [ ] sideBarBorder
                                      [ ] sashHover
                                      [ ] statusAndTitleBorders
                                      [ ] debuggingStatusBar
                                      [ ] tabActiveBorder

[Confirm]      Delete peacock settings from .vscode/settings.json? (default Yes)
[Confirm]      Open with `code` after creation? (default Yes)

[Confirm]      Advanced options? (default No)
               └ Yes면:
                 [Confirm] keepForegroundColor
                 [Confirm] keepBadgeColor
                 [Confirm] squigglyBeGone
                 [Input]   darkenLightenPct (default 10)

[Review]       요약 + 최종 Confirm
```

## 9. 열기

기본: `code <workspace-file>` (fork, 부모 exit 대기 안 함). `--no-open`이면 스킵.

`code`가 PATH에 없는 경우: stderr 경고 + workspace 파일 경로를 stdout에 출력해서 copy-paste 가능하게. 종료 코드는 0 (파일 생성은 성공).

## 10. 패키지 구조

```
vscode-color-workspace/
├── cmd/ccws/main.go              # cobra root, 서브커맨드 등록
├── internal/
│   ├── color/
│   │   ├── primitives.go         # parse, isLight, lighten, darken, alpha, triad, luminance, contrast
│   │   ├── palette.go            # Palette(base, opts) → map[string]string
│   │   └── palette_test.go       # golden (peacock fixture 비교)
│   ├── peacock/
│   │   └── keys.go               # ColorSettings 28키 const + peacock.* 세팅명
│   ├── workspace/
│   │   ├── workspace.go          # JSONC read, merge, atomic write, guard 1
│   │   └── workspace_test.go
│   ├── vscodesettings/
│   │   ├── settings.go           # .vscode/settings.json 정밀 삭제, guard 2, 빈 구조 정리
│   │   └── settings_test.go
│   ├── interactive/
│   │   └── form.go               # huh Form 구성
│   └── runner/
│       ├── options.go            # Options struct, 기본값
│       ├── runner.go             # 오케스트레이션
│       └── runner_test.go
├── go.mod
└── README.md
```

**의존성**:
- `github.com/spf13/cobra` — 서브커맨드
- `github.com/charmbracelet/huh` — interactive form
- `github.com/lucasb-eyer/go-colorful` — HSL/RGB 변환
- `github.com/tailscale/hujson` — JSONC 파싱

**JSONC 취급**: `hujson.Parse` → `hujson.Standardize` → `encoding/json.Unmarshal`. 쓰기는 표준 JSON으로 (2-space 들여쓰기). 원본 주석 보존은 v1 목표 아님 — 문서화.

**Atomic write**: temp 파일 → `os.Rename`.

## 11. Exit codes & 에러 UX

| Code | 의미 |
|---|---|
| 0 | 성공 |
| 1 | 입력 에러 (존재하지 않는 폴더, 잘못된 색, JSONC 구문 오류) |
| 2 | Safety guard 트리거 |
| 3 | 파일시스템 에러 (권한, 쓰기 실패) |

**Stderr 경고 (exit 0 유지)**:
- Parent가 git repo — `.code-workspace` 커밋 우려
- `code` PATH에 없음 — 파일 경로만 출력

## 12. 테스트

**`internal/color/palette_test.go`**: 고정 base 5개(`#ff0000`, `#42b883`, `#5a3b8c`, `#000000`, `#ffffff`) × 기본 affects → Peacock fixture와 28키 완전 일치. Fixture는 `testdata/`에 JSON.

**`internal/workspace/workspace_test.go`**:
- 없던 파일 새로 생성 (folders/settings 구조)
- 기존 파일 + non-peacock 키만 있음 → merge 성공
- 기존 파일 + peacock 키 존재 → Guard 1 에러 (파일 건들지 않음 확인)
- 기존 파일 + peacock 키 + `--force` → overwrite 성공
- JSONC 주석 포함 input 파싱
- folders/launch/extensions/tasks 다른 키 보존

**`internal/vscodesettings/settings_test.go`**:
- peacock.* + 팀 설정(`editor.tabSize`) 혼재 → peacock.* 만 삭제
- colorCustomizations에 peacock 28키 + custom 키 → Guard 2 에러 (파일 건들지 않음 확인)
- colorCustomizations에 peacock 키만 → 삭제 후 그 키 사라짐
- colorCustomizations 비면 그 키 자체 삭제
- settings.json 비면 파일 삭제, `.vscode/` 빈 디렉터리도 삭제
- 파일 없을 때 no-op

**`internal/runner/runner_test.go`**: scenario 테이블 (existing workspace 유/무 × peacock in settings.json 유/무 × `--force` 유/무 × non-peacock colorCustomizations 유/무). `t.TempDir`로 격리. `code` 실행은 주입된 `Opener` interface로 mock.

## 13. Open questions

v1 스펙 범위에 없지만 추후 논의 가능:
- Peacock favorites import (`peacock.favoriteColors` → 선택 메뉴)
- Advanced options을 dotfile (`~/.config/ccws.toml`)로 default 저장
- Multi-root workspace 지원
