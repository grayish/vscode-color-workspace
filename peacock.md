# Peacock Local Settings 우회 - 조사 정리

Peacock이 `.vscode/settings.json`에 색 설정을 영구 저장하는 바람에 Git에 섞여드는 문제를 우회하는 방법 조사. 최종적으로 **`.code-workspace` 파일 기반 CLI**를 만드는 것이 목표.

## 1. 문제 정의

[Peacock](https://github.com/johnpapa/vscode-peacock)은 "내 VSCode 창이 어느 프로젝트인지 한눈에 식별"용 개인 도구인데, 설정 저장 위치가 `.vscode/settings.json`이라 Git 공유 경로에 들어감. 개인 설정과 팀 공유 설정이 같은 파일에 섞이는 구조적 부조화.

Peacock 이슈 트래커에는 [#7](https://github.com/johnpapa/vscode-peacock/issues/7)(2019) / [#282](https://github.com/johnpapa/vscode-peacock/issues/282) / [#410](https://github.com/johnpapa/vscode-peacock/issues/410) / [#512](https://github.com/johnpapa/vscode-peacock/issues/512) / [#517](https://github.com/johnpapa/vscode-peacock/issues/517) 로 같은 요청이 7년째 반복되고 있으나 제작자가 "프로젝트별 색상"이라는 철학을 고수해 모두 open 상태.

## 2. VSCode 설정 아키텍처 (핵심)

### 저장 타겟

VSCode Extension API의 `ConfigurationTarget` enum:
```
Global          → User settings (혹은 활성 Profile의 settings.json)
Workspace       → .vscode/settings.json 또는 .code-workspace의 "settings"
WorkspaceFolder → multi-root의 각 폴더별 .vscode/settings.json
```

**"Memory"/"Runtime only" 타겟은 존재하지 않음.** 모든 설정값은 반드시 파일에 persist됨 — extension이 런타임에 값만 바꾸고 저장 안 하는 것은 API 레벨에서 불가능.

### Folder open vs Workspace file open

| | Folder open (`code /path`) | Workspace open (`code /path.code-workspace`) |
|---|---|---|
| `workspace.workspaceFile` | `undefined` | `.code-workspace` 파일 경로 |
| 설정 계층 | 3단: Default → User → Workspace | 4단: Default → User → Workspace(파일) → WorkspaceFolder |
| `Target.Workspace` 저장 대상 | `.vscode/settings.json` | **`.code-workspace`의 `"settings"` 객체** |

**→ Workspace file 모드가 되면 `ConfigurationTarget.Workspace` 저장 대상이 바뀐다.** 이 동작이 이 프로젝트의 근본 레버리지.

## 3. 기각한 대안들

| 방법 | 이유 |
|---|---|
| `settings.local.json` (VSCode Issue [#40233](https://github.com/microsoft/vscode/issues/40233), [#38902](https://github.com/microsoft/vscode/issues/38902)) | 공식 out-of-scope, 9년째 미구현 |
| Workspace settings 전체 opt-out ([#206802](https://github.com/microsoft/vscode/issues/206802)) | 공식 out-of-scope |
| User settings override workspace ([#210930](https://github.com/microsoft/vscode/issues/210930), [#292016](https://github.com/microsoft/vscode/issues/292016)) | 미구현 |
| User settings에 colorCustomizations 넣기 | 모든 창 동일 색 → 프로젝트별 구분 상실 |
| VSCode Profiles | Folder→Profile 이 **many-to-one** — profile을 공유한 folder들은 같은 색 공유. 진정한 per-folder 색 위해선 프로젝트마다 profile 생성해야 해서 비현실적 (profile은 extension/keybinding까지 격리하는 무거운 단위) |
| CSS injection (Custom CSS/JS Loader) | VSCode 내부 파일 패치 → 매 실행 시 "corrupted" 경고, 업데이트마다 재패치 필요 |
| Peacock fork | 근본 원인이 VSCode API 한계라 fork 해도 동일 |

## 4. 채택한 해결책: `.code-workspace` 파일

### 조건 충족

| 조건 | 충족 |
|---|---|
| 프로젝트별 다른 색 | ✅ |
| `.vscode/settings.json` 미수정 | ✅ |
| 실용적 | ✅ (단, 폴더 직접 열기는 포기, `.code-workspace`로 열어야 함) |

### 생성할 JSON 구조

```json
{
  "folders": [
    { "path": "." }
  ],
  "settings": {
    "peacock.color": "#5a3b8c",
    "workbench.colorCustomizations": {
      "activityBar.activeBackground": "#7144a8",
      "activityBar.background": "#5a3b8c",
      "activityBar.foreground": "#e7e7e7",
      "activityBar.inactiveForeground": "#e7e7e799",
      "activityBarBadge.background": "#eb7cff",
      "activityBarBadge.foreground": "#15202b",
      "statusBar.background": "#452d6c",
      "statusBar.foreground": "#e7e7e7",
      "statusBarItem.hoverBackground": "#5a3b8c",
      "statusBarItem.remoteBackground": "#452d6c",
      "statusBarItem.remoteForeground": "#e7e7e7",
      "titleBar.activeBackground": "#452d6c",
      "titleBar.activeForeground": "#e7e7e7",
      "titleBar.inactiveBackground": "#452d6c99",
      "titleBar.inactiveForeground": "#e7e7e799"
    }
  }
}
```

### 배치 옵션

| 위치 | 장점 | 단점 |
|---|---|---|
| Repo 밖 (`~/workspaces/<name>.code-workspace`) | 실수 커밋 불가능 | 파일 관리 별도 |
| **Repo 내 + `.git/info/exclude`** (권장) | 프로젝트 폴더에 자연스레 위치, 팀 `.gitignore` 미수정 | 머신 간 재생성 필요 |
| Repo 내 + `.gitignore` | 팀이 같은 방식 공유 가능 | 팀 합의 필요 |

### 여는 방법
- CLI: `code path/to/<name>.code-workspace`
- Finder: `.code-workspace` 확장자 handler가 VSCode이면 더블클릭
- VSCode 내부: `File > Open Workspace from File...`
- 한 번 열면 Recent Workspaces에 등록되어 이후 편리

## 5. CLI 설계 요구사항

### 입력
- 대상 폴더 경로 (기본값: `$PWD`)
- 색상 (hex `#RRGGBB` 또는 CSS 명명 색상)
- 파일명 (기본값: 폴더명 기반, 예: `asr-tts-reaseach.code-workspace`)
- 배치 위치 플래그: `--in-repo` (기본) / `--external <dir>`

### 색상 → `workbench.colorCustomizations` 변환

Peacock 알고리즘 참고: https://github.com/johnpapa/vscode-peacock/blob/main/src/color-library.ts
- 배경색은 입력 색 그대로
- 전경색은 배경 밝기에 따라 흰색/검정 중 선택 (W3C contrast)
- `activityBar.activeBackground`, `activityBarBadge.background` 등은 hue/saturation 조정한 파생색

간단 MVP는 배경만 사용자 입력, 전경은 대비색으로 고정해도 충분.

### 영향 범위 (Peacock 기본 affectedElements)
- `activityBar` (좌측 세로 바)
- `statusBar` (하단)
- `titleBar` (상단, `window.titleBarStyle: "custom"` 필요)

옵션: `editorGroup.border`, `panel.border`, `sash.hoverBorder`

### Gitignore 자동 처리
서브 커맨드 또는 플래그로:
- `.git/info/exclude`에 자동 추가 (기본)
- `--use-gitignore`: `.gitignore` 수정
- `--no-ignore`: 건드리지 않음

### 멱등성 (재실행 안전)
기존 `.code-workspace` 존재 시:
- **전체 overwrite 금지** — `folders`, `launch`, `tasks`, `extensions` 등 다른 설정이 있을 수 있음
- `settings.workbench.colorCustomizations` 와 `settings.peacock.color` 만 덮어쓰기
- 나머지 키는 보존

### 사전 검사
- 대상 폴더가 존재하는지
- Git repo인지 (gitignore 처리용)
- `.vscode/settings.json`에 `workbench.colorCustomizations`가 이미 있는지 → **workspace file settings 보다 우선순위 높아 효과 없음**. 이 경우 경고 또는 해당 키 제거 제안
- `.code-workspace`의 `folders[0].path`가 대상 폴더를 가리키는지

### 열기 자동화 (옵션)
`--open` 플래그: 생성 후 `code <file.code-workspace>` 자동 실행

## 6. Gotcha

1. **`.vscode/settings.json`의 `workbench.colorCustomizations`가 workspace 파일을 override** — `.vscode/settings.json` (WorkspaceFolder scope) > `.code-workspace` (Workspace scope). 기존에 colorCustomizations가 박혀있으면 workspace 파일에 뭘 써도 안 먹힘.
2. **폴더 직접 열기는 작동 안 함** — 반드시 `.code-workspace` 경로로 열어야 workspace 모드. 폴더만 열면 여전히 `.vscode/settings.json`이 Workspace target.
3. **`titleBar.activeBackground` 효과는 `window.titleBarStyle: "custom"`에서만** — native title bar는 OS가 칠하므로 무시됨.
4. **`folders[].path`는 workspace 파일 기준 상대 경로** — 파일 이동 시 깨짐.
5. **Recent Workspaces 캐시** — 같은 경로로 이전에 folder-open 한 적 있으면 구분해서 인식.

## 참고 자료

- [VS Code — User and Workspace Settings](https://code.visualstudio.com/docs/configure/settings)
- [VS Code — Profiles](https://code.visualstudio.com/docs/configure/profiles)
- [VS Code — Multi-root Workspaces](https://code.visualstudio.com/docs/editor/multi-root-workspaces)
- [Peacock 소스 — color-library.ts](https://github.com/johnpapa/vscode-peacock/blob/main/src/color-library.ts)
- [Simon Porter — Managing Peacock colours in shared settings](https://www.simonporter.co.uk/posts/managing-peacock-colours-in-shared-settings/)
