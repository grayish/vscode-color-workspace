# Worktree main `--force` propagation — Design

## 1. 문제와 목표

현재 `ccws --force`는 main 워크트리에서 실행할 때 두 가지 문제가 있다:

1. **단일 worktree repo에서 거짓 라벨**: `git worktree list --porcelain`은 일반 git repo에서도 1개 entry(자기 자신, IsMain=true)를 반환한다. 따라서 `.code-workspace`에 색이 이미 있는 일반 repo에서 `ccws --force`를 재실행하면 Case A 진입 → `IdentityHash(self.IsMain=true) == 0` → `LadderOffset(0) == 0` → 같은 색이 재기록되며, 결과 라벨이 `from worktree family`로 표시된다. 그러나 family 자체가 없는 상황이므로 misleading.

2. **multi-worktree main에서 family 전환 의도가 구현되지 않음**: 사용자의 mental model은 "main에서 `--force` 실행 = 이 패밀리 전체의 색을 전환하겠다는 의미". 그러나 현재 Case A는 main에서도 offset 0으로 main 색을 그대로 보존만 하고, linked 워크트리들의 `.code-workspace`는 건드리지 않는다 → main 측 `--force`는 사실상 no-op.

목표:
- 단일-worktree main + `--force`: 새 random/`--color` 색으로 재생성, `from worktree family` 라벨 제거.
- multi-worktree main + `--force`: anchor를 새로 결정 (random 또는 `--color`), 그 anchor를 main에 적용 + 모든 **이미 family에 가입된** linked 워크트리의 `.code-workspace`에 derived 색 propagate.
- linked + `--force`: 변화 없음 (기존 Case A 동작 유지).

이 변경은 `2026-04-27-worktree-similar-color-design.md`의 Case A를 세분화하고, "main이 family의 authority"라는 원칙을 명시한다.

Non-goals:
- linked 워크트리에서 `--force` 실행 시 family 전체 전환 (linked는 sub 멤버이므로 권한 없음).
- linked 워크트리의 `.code-workspace`를 새로 **생성** (가입은 사용자가 해당 워크트리에서 직접 ccws 실행하는 것으로 정의).
- 색 전환 atomicity 보장 (best-effort: 부분 실패 가능, exit 1로 알림).
- propagation 시 element adjustment(`peacock.affectActivityBar` 등) 옵션 변화 (각 워크트리는 자기 옵션 유지).

## 2. 새 동작 매트릭스

T = target, M = main 워크트리, L = linked 워크트리들. `ws(W)` = `<parent>/<basename(W)>.code-workspace`.

기존 매트릭스 (`2026-04-27-...`)에서 **Case A를 분기**하고 새로운 propagation 동작을 추가한다.

| 케이스 | 조건 | 동작 |
|---|---|---|
| **A1** (변경) | T == M, len(worktrees) == 1, `--force` | 워크트리 로직 skip → fall through (T의 settings.json → 랜덤). `SourceSettings` 또는 `SourceRandom`. **거짓 라벨 제거**. |
| **A2** (변경) | T == M, len(worktrees) > 1, `--force` | 새 anchor 결정 (`--color` 우선, 없으면 random). M 갱신. **family 가입된 모든 L에 derived 색 propagate**. `SourceWorktree`. warn으로 갱신/스킵/실패 보고. |
| **A3** (기존 Case A 유지) | T ∈ L, M에 색 있음 | 기존 동작 그대로. `IdentityHash(T)` offset 적용. M 안 건드림. `SourceWorktree`. |
| **B** (기존 유지) | T == M, M·L 모두 색 없음 | 기존 동작 그대로. fall through. |
| **C** (기존 유지) | T ∈ L, M·다른 L 모두 색 없음 | 기존 동작 그대로. M anchor 자동 작성, T에 derived 적용. |
| **D** (기존 유지, 부수 효과 있음) | M에 색 없음, L 중 누군가 색 있음 | 기존 동작 그대로 (T가 main이든 linked든 family disabled, fallback). 단 T == M에서 `--force`로 진입하면 A2가 트리거되어 새 anchor가 family를 정상화한다 (의도된 부수 효과). |
| 비-git, `git worktree list` 실패 | — | 기존 동작 그대로 (silent skip). |

A1이 발동하려면 `--force`가 필요하다. `--force` 없이 `.code-workspace`에 peacock 키가 있으면 `runner.Run`의 preconfigured short-circuit이 먼저 잡아 그냥 열기만 한다 (변화 없음).

A2에서 "family 가입된 L"의 정의:
- `ws(L)` 파일이 존재하고 + 그 파일에 적어도 하나의 peacock 키(`peacock.color` 또는 element 색 키)가 있음.
- 둘 중 하나라도 빠진 L은 **skip** (warn에 명시).

근거: 가입의 의미적 트리거는 "그 워크트리에서 ccws를 한 번이라도 실행한 흔적". `.code-workspace`만 있고 peacock 키가 없는 경우는 사용자가 수동으로 만든 `.code-workspace`(폴더/태스크용)일 가능성이 높아 propagation으로 peacock 키를 끼워 넣는 것은 surprising하다.

## 3. 우선순위 체인 (변화 없음)

```
1. --color 플래그              → SourceFlag (단, A2에서는 anchor로 사용되어 propagate)
2. 워크트리 로직 (Cases A2/A3/C)→ SourceWorktree
3. T의 .vscode/settings.json    → SourceSettings  (A1/B/D fallback)
4. color.Random()              → SourceRandom    (A1/B/D fallback)
```

A2는 `--color`를 우회하지 않고 **anchor 결정 단계에서 사용한다**: anchor = `--color` 값(있으면) 또는 random. 이후 main 색이 이 anchor가 되고, linked 색은 anchor + offset(L). `--color`를 명시했어도 `SourceWorktree`로 보고한다 (family 전환 동작이 일어났음을 라벨이 반영).

A1은 `--color`가 있으면 `SourceFlag`로 빠진다 (단일-worktree main에는 propagate할 family 자체가 없으므로 A2 분기가 아니다).

## 4. 렌더링된 모양

**A1 (단일-worktree main + `--force`):** 변화 없음. 기존 `ok workspace ready` 배지. `from worktree family` 라벨 더 이상 안 나옴.

**A2 (multi-worktree main + `--force`, 전체 성공):**
```
  warn  family propagated from main worktree
        anchor at  ~/code/myproj.code-workspace      #5a3b8c
        applied    ~/code/myproj-feat-x.code-workspace   #6747a4
                   ~/code/myproj-bugfix.code-workspace  #4a2b6c
        skipped    ~/code/myproj-hotfix             (no peacock keys)
  ok    workspace ready
        ~/code/myproj.code-workspace  #5a3b8c
```

`applied`/`skipped`/`failed` 각 섹션은 항목이 있을 때만 표시. 셋 다 비어 있으면 (즉 family 가입 linked가 0개) 헤더 + `(no linked worktrees in family)` 한 줄로 사용자에게 propagation 시도가 있었음을 알림.

**A2 (부분 실패):**
```
  warn  family propagated from main worktree
        anchor at  ~/code/myproj.code-workspace      #5a3b8c
        applied    ~/code/myproj-feat-x.code-workspace   #6747a4
        failed     ~/code/myproj-bugfix.code-workspace  permission denied
        skipped    ~/code/myproj-hotfix             (no peacock keys)
  ok    workspace ready
        ~/code/myproj.code-workspace  #5a3b8c
```

`failed` 섹션이 있으면 종료 코드 1. main 갱신 자체가 실패하면 hard error (exit 1, ok 배지 안 나옴). `failed` 텍스트는 짧은 사유 (예: `permission denied`, `disk full`, `parse error`).

순서: applied → failed → skipped. failed가 있으면 사용자가 가장 먼저 봐야 하는 정보지만 applied는 "성공한 것" 컨텍스트로 먼저 보여주고, 그다음 failed로 주의를 끄는 흐름.

**B/C/D**: 변화 없음.

## 5. 패키지 구조

기존 구조 유지. 새 파일 추가 없음. 기존 파일 수정만:

```
internal/
├── runner/
│   ├── resolve.go      ← Case A 분기 재구성, PropagateIntent 신규
│   ├── runner.go       ← writeFamilyPropagation 헬퍼, Run 확장
│   ├── resolve_test.go ← Case A 테스트 갱신 + A1/A2 테스트 추가
│   └── runner_test.go  ← propagation 통합 테스트 추가
└── ...
cmd/ccws/
├── render.go           ← A2 warn 렌더링 추가
└── render_test.go      ← 추가 테스트
```

DAG 변경 없음.

## 6. Runner 레이어 변경

### 6.1 `PropagateIntent` 신규

`AnchorIntent`(Case C용)와 별도로 propagation 의도를 표현:

```go
// PropagateIntent describes A2 side effects: write the anchor color to main's
// .code-workspace and write each derived color to the corresponding linked
// worktree's .code-workspace. The runner executes the writes; resolve only
// computes the targets.
type PropagateIntent struct {
    AnchorPath  string         // ws(M)
    AnchorColor color.Color
    Targets     []PropagateTarget
    Skipped     []SkippedLinked // ws paths not in family, with reason
}

type PropagateTarget struct {
    WorkspacePath string       // ws(L) for some linked L
    DerivedColor  color.Color  // AnchorColor.ApplyLightness(LadderOffset(IdentityHash(L)))
}

type SkippedLinked struct {
    WorkspacePath string
    Reason        string       // e.g., "no .code-workspace", "no peacock keys", "parse error: ..."
}
```

`AnchorIntent`는 그대로 유지 (Case C는 단일 외부 파일 작성만 필요하고 propagation과 의미가 다름).

### 6.2 `ResolveColor` 시그니처 확장

```go
func ResolveColor(
    targetDir, flag string, force, debug bool,
) (color.Color, ColorSource, []string, *AnchorIntent, *PropagateIntent, error)
```

`force` 파라미터 추가 — A1/A2 분기는 `--force` 플래그가 있어야만 발동. (A1은 short-circuit이 이미 차단하지만 명시적 인자로 받아 의도를 코드로 표현.)

`AnchorIntent`와 `PropagateIntent` 중 한 번에 하나만 non-nil.

### 6.3 `resolveFromWorktree` 분기 재구성

```go
// 의사 코드
self := FindSelf(...)
main := worktrees[0]
mainColor := readWorkspacePeacockColor(ws(main))

switch {
case mainColor != nil && self.IsMain && len(worktrees) == 1:
    // A1: 단일 worktree main + 색 있음 + force (force 없으면 short-circuit이 잡음)
    return ColorZero, 0, nil, nil, nil, false  // fall through

case mainColor != nil && self.IsMain && len(worktrees) > 1 && force:
    // A2: multi-worktree main + force → propagate
    var anchor color.Color
    if flag != "" {
        anchor, err = color.Parse(flag)  // 에러는 이미 ResolveColor 도입부에서 검증됨
    } else {
        anchor = color.Random()
    }
    targets, skipped := buildPropagateTargets(worktrees, anchor)
    intent := &PropagateIntent{
        AnchorPath: ws(main), AnchorColor: anchor,
        Targets: targets, Skipped: skipped,
    }
    // warn 문자열은 runner.Run이 propagation 실행 후 만든다 (write-time failures 포함)
    return anchor, SourceWorktree, nil, nil, intent, true

case mainColor != nil && !self.IsMain:
    // A3: linked + main has color → 기존 Case A
    offset := LadderOffset(IdentityHash(self))
    derived := mainColor.ApplyLightness(offset)
    return derived, SourceWorktree, nil, nil, nil, true

// 그 외: B/C/D 기존 흐름 유지
}
```

A2에서 anchor 결정 후 main에 그 색을 그대로 쓰는데, main의 IdentityHash는 0이라 offset 0 → derived = anchor. main과 linked가 같은 anchor를 기준으로 일관된 family를 형성한다.

### 6.4 `--color` flag 처리 변경

기존: `flag != ""` → 즉시 SourceFlag로 반환 (워크트리 로직 우회).
변경: 워크트리 로직을 먼저 시도하고, A2가 발동하면 flag를 anchor로 사용. A2가 발동하지 않으면 (단일-worktree main, linked target 등) 기존대로 SourceFlag.

```go
func ResolveColor(targetDir, flag string, force, debug bool) (...) {
    // flag 유효성만 먼저 검증 (A2 진입해도 같은 색을 써야 하므로)
    var parsed color.Color
    if flag != "" {
        parsed, err = color.Parse(flag)
        if err != nil { return ..., fmt.Errorf("--color: %w", err) }
    }

    // 워크트리 로직 시도 (flag, force 모두 전달)
    c, src, warns, anchorIntent, propagateIntent, ok, err := resolveFromWorktree(targetDir, flag, force, debug)
    if err != nil { return ..., err }
    if ok {
        return c, src, warns, anchorIntent, propagateIntent, nil
    }

    // 워크트리 로직이 결정 안 함 → flag 우선
    if flag != "" {
        return parsed, SourceFlag, warns, nil, nil, nil
    }

    // flag도 없음 → settings.json → random
    ...
}
```

`resolveFromWorktree`는 A2 분기 안에서 flag를 anchor로 채택 (§6.3 코드 참고). flag가 빈 문자열이면 `color.Random()` 사용.

### 6.5 `runner.Run` 변경

```go
// 의사 코드
c, src, warns, anchorIntent, propagateIntent, err := ResolveColor(abs, flag, opts.Force, opts.Debug)

if anchorIntent != nil {
    if err := writeAnchorWorkspace(anchorIntent, opts); err != nil { return ..., err }
}
if propagateIntent != nil {
    propagateResult, err := writeFamilyPropagation(propagateIntent, opts)
    if err != nil { return nil, err }  // main 쓰기 자체 실패 = hard error
    // propagateResult.Failed에 부분 실패 정보가 모임
    res.PropagatedTo = paths(propagateResult.Applied)
    res.SkippedLinked = propagateIntent.Skipped
    res.FailedLinked = propagateResult.Failed
}
```

`writeFamilyPropagation` 시그니처:

```go
type PropagateResult struct {
    Applied []string                // ws paths written successfully
    Failed  []PropagateFailure      // ws paths where write failed, with reason
}

type PropagateFailure struct {
    WorkspacePath string
    Err           error
}

// writeFamilyPropagation writes the anchor to main and derived colors to each
// linked target. Returns hard error if main write fails. Linked write failures
// are accumulated in PropagateResult.Failed and do not abort the loop.
func writeFamilyPropagation(intent *PropagateIntent, opts Options) (PropagateResult, error)
```

main 작성을 가장 먼저 시도. 실패하면 hard error로 반환 (linked 시도 안 함 — anchor가 없으면 family 갱신이 의미 없음). main 성공 후 linked 순회, 각각 실패 수집. 모든 linked 시도 후 반환.

### 6.6 `Result` 필드 추가

```go
type Result struct {
    // 기존 필드들 ...
    PropagatedTo  []string             // A2에서 갱신 성공한 linked ws paths
    SkippedLinked []SkippedLinked      // A2에서 family 비가입/parse 에러로 건너뛴 linked
    FailedLinked  []PropagateFailure   // A2에서 write 실패한 linked
}
```

Exit code 메커니즘:
- `runner.Run`은 propagation 부분 실패 시 `(populatedResult, ErrPartialPropagation)`을 반환하는 새 패턴을 채택. (기존 hard error는 `(nil, err)` 패턴 유지.)
- `cmd/ccws/root.go` RunE는 err가 `ErrPartialPropagation`일 때 res로 warnings 렌더링 후 err를 반환 → `errToExit`가 기본 exit 1로 매핑.
- 새 sentinel:
  ```go
  // runner/errors.go (또는 runner.go)
  var ErrPartialPropagation = errors.New("runner: family propagation had failures")
  ```
- 기타 매핑은 변화 없음.

## 7. 렌더링 (`cmd/ccws/render.go`)

기존 `renderWarnings` 경로 재사용. propagation warn 문자열은 multi-line 포맷이라 별도 렌더링 분기 불필요.

`resolveFromWorktree`는 warn 문자열을 만들지 않는다 (§6.3). 대신 `runner.Run`이 `writeFamilyPropagation` 결과를 받은 뒤 한 번에 final warn을 합성한다 — write-time failures를 포함해야 하므로:

```go
// resolve.go
func formatPropagatedWarning(intent *PropagateIntent, failed []PropagateFailure) string

// runner.go (Run 안)
warn := formatPropagatedWarning(propagateIntent, propagateResult.Failed)
warnings = append(warnings, warn)
```

`formatPropagatedWarning`의 출력은 §4 렌더링 모양과 일치.

## 8. 테스트 전략

### 8.1 `internal/runner/resolve_test.go`

기존 갱신:
- `TestResolveColor_WorktreeCaseA_MainTarget` (line 176): 단일 worktree main + 기존 색 → 현재 SourceWorktree 검증. **A1 동작에 맞게 SourceSettings/Random 검증으로 갱신**.

신규:
- `TestResolveColor_A2_MainForce_NoColor`: multi-worktree main + force, --color 없음 → SourceWorktree, PropagateIntent에 anchor + targets 포함.
- `TestResolveColor_A2_MainForce_WithColor`: multi-worktree main + force, --color X → anchor가 X, propagate.
- `TestResolveColor_A2_SkipsLinkedWithoutPeacock`: linked가 .code-workspace 없음/peacock 키 없음 → Skipped에 포함, Targets에서 제외.
- `TestResolveColor_A1_SingleWorktreeMain_Force_FallsThrough`: 단일 worktree main + force + 기존 색 → ok=false, fall through.
- `TestResolveColor_A1_NoForce_Unchanged`: 단일 worktree main + 기존 색 + force=false → 기존 흐름. (실제로 short-circuit이 잡지만 resolve 레벨 호출 시 동작 확인.)
- `TestResolveColor_A3_LinkedTarget_Unchanged`: linked + force → 기존 Case A 동작 그대로.

### 8.2 `internal/runner/runner_test.go`

신규:
- `TestRun_A2_PropagatesToFamilyMembers`: 임시 디렉토리에 main + 2개 linked .code-workspace 생성 (peacock 색 포함). main에서 ccws --force 실행. main 색 변경, linked 색도 derived로 변경 확인.
- `TestRun_A2_SkipsUncoloredLinked`: linked 중 하나는 peacock 없음. Skipped에 포함, 그 파일은 안 건드림.
- `TestRun_A2_PropagationPartialFailure`: linked 하나의 .code-workspace를 chmod 0444로 권한 막음. main 갱신 성공, 그 linked는 FailedLinked에, 나머지는 Applied. Exit 1 매핑.
- `TestRun_A2_MainWriteFails`: main 갱신 실패 → hard error, linked 안 시도.

### 8.3 `cmd/ccws/render_test.go`

신규:
- `TestRenderWarning_FamilyPropagated_AllSuccess`: warn 문자열의 헤더 + applied + skipped 표시 검증.
- `TestRenderWarning_FamilyPropagated_PartialFailure`: applied + failed + skipped 셋 다 표시.
- `TestRenderWarning_FamilyPropagated_NoLinkedInFamily`: applied/skipped/failed 모두 빈 경우 헤더 + `(no linked worktrees in family)`.

## 9. 백워드 호환

- linked + `--force` 동작 변화 없음 (A3).
- main + `--force` 단일 repo 동작 변화: 기존엔 같은 색 보존 → 신규엔 random/`--color`로 재생성. **사용자 가시 변화**. spec 단계에서 "단일 repo에서 `--force`로 재생성"을 의도된 동작으로 정의 (현재 동작은 우연한 부수 효과).
- main + `--force` multi-worktree 동작 변화: 기존엔 같은 색 보존 → 신규엔 anchor 새로 결정 + linked propagate. **사용자 가시 변화**. mental model에 부합.
- main 비고 linked 색 있는 D 케이스: T == M에서 `--force` 실행 시 A2가 anchor를 새로 세우면서 linked 덮어쓰기 → family disabled 상태에서 자동 회복. 의도된 부수 효과로 문서화.
- 기존 `--color` 동작: 단일/linked 타겟에서는 그대로 SourceFlag 라벨. multi-worktree main + force에서만 A2 anchor로 사용 (SourceWorktree 라벨).

`AnchorIntent`/Case C 동작 변화 없음.

## 10. 실패 모드와 처리

| 상황 | 처리 |
|---|---|
| main 색 결정 실패 (`--color` parse 에러) | 기존 hard error (exit 1) |
| main `.code-workspace` write 실패 | hard error (exit 1, "write main anchor workspace" 컨텍스트). linked 시도 안 함. |
| linked `.code-workspace` write 실패 | `Result.FailedLinked`에 수집, 다음 linked 계속 시도. 전체 끝나고 main과 다른 linked는 적용된 상태로 종료. exit 1. |
| linked `.code-workspace` parse 실패 (기존 파일이 깨진 경우) | `Skipped`에 `Reason="parse error: <detail>"`로 분류. 깨진 파일을 조용히 무시하지 않고 사용자에게 명시적으로 보고. write를 시도하지 않으므로 Failed가 아닌 Skipped 카테고리. |
| linked 디렉토리가 git에 등록되어 있지만 디스크에 없음 | `gitworktree.List`가 `readGitDirPointer`에서 실패 → 전체 ErrNotInWorktree로 fall through (현재 동작). A2 자체가 발동 안 함. |
| `git worktree list` 자체 실패 | 기존 silent skip → A1/A2 모두 미발동 → 기존 fall through. |

`PropagateFailure.Err`은 사용자에게 짧은 사유로 표시 (`permission denied`, `parse error: <detail>` 등). full error는 debug 로그에만.

## 11. 보안/멱등성

- 같은 인자로 두 번 실행: `--color` 명시 시 멱등 (anchor 동일 → 모든 derived 동일). `--color` 없이 random일 때는 매번 새 색.
- linked의 `.code-workspace`에 peacock 외 사용자 설정이 있을 때: `workspace.ApplyPeacock`은 peacock 키만 갱신, 다른 설정 보존 (기존 동작).
- 외부 디렉토리 쓰기 범위: linked 워크트리의 `<parent>/<basename>.code-workspace` 파일만 — 기존 Case C와 같은 정책. `.vscode/settings.json`은 손대지 않음.
- 동시 실행: 파일 수준 락 없음. 두 ccws 프로세스가 동시에 같은 family에 작용하면 마지막 쓰기가 이김. 흔한 시나리오는 아니므로 mitigation 불필요.

## 12. 메시지/문서 업데이트

- `README.md`: `--force` 설명에 main + multi-worktree에서의 propagation 동작 한 단락 추가.
- `CLAUDE.md`: "Safety guards" 섹션에 propagation 동작이 외부 dir 쓰기를 한다는 점 명시.
