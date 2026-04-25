# Claude: ccws maintenance notes

Go CLI that replaces Peacock VSCode extension by writing colors to `<parent>/<folder>.code-workspace` instead of the shared `.vscode/settings.json`. User-facing info in `README.md`; design rationale in `docs/superpowers/specs/` and `docs/superpowers/plans/`.

## Commands — use Taskfile, not raw `go`

```bash
task            # test
task build      # → ./ccws
task install    # → $GOBIN/ccws
task test:race  # race detector
task lint       # vet + gofmt check (CI runs this)
task ci         # lint + test:race
task fixture    # regenerate golden fixture (needs Node)
task --list     # full target list
```

Always run `task lint` before committing — past sessions have landed gofmt-dirty code because subagents skipped formatting.

## External Peacock reference

Color algorithm was ported from `/Users/user/Projects/vscode-peacock/` (read-only, NOT a submodule). When modifying palette logic, cross-reference:

- `src/color-library.ts` — primitives (tinycolor2 usage)
- `src/configuration/read-configuration.ts` — `prepareColors`, `collect*Settings`
- `src/models/enums.ts` — `ColorSettings` enum (29 keys — not 28, spec has a typo)

Parity is enforced by `internal/color/golden_test.go` against 6 fixtures (5 plain bases + 1 with element adjustments) at `internal/color/testdata/fixture.json`, generated from `scripts/gen-peacock-fixture/main.js`. When modifying palette logic, update both the Go code AND the Node generator, then `task fixture` — otherwise fixtures diverge.

## Test isolation — `DefaultOptions()` now carries non-zero Adjust

`DefaultOptions()` returns `Adjust: {ActivityBar: Lighten, TitleBar: Darken, StatusBar: None}` so bars differ by default. Tests that assert a bar or accent border equals the base color must explicitly reset: `opts.Adjust = AdjustOptions{}`. Forgetting this is how `TestCollectTitleBar_Defaults` / `TestCollectAccentBorder` can mysteriously regress.

## Safety guards (project-specific terminology)

Used in error messages, tests, and commit messages:

- **Guard 1** — existing Peacock keys in the target `.code-workspace` (would overwrite).
- **Guard 2** — non-Peacock keys would remain in `.vscode/settings.json` after cleanup.
- Either triggers → exit code **2**. `--force` bypasses **both** (single flag by design).

## Exit codes

`0` success · `1` input error · `2` safety guard · `3` filesystem error. Mapping lives in `cmd/ccws/root.go:errToExit`; interactive mode converts Guard errors into huh confirms, re-runs with `opts.Force = true` on accept.

## Package import rule

Strict DAG — no cycles, no backward edges:

```
color → (stdlib only)
peacock → (stdlib only)
jsonc → hujson
tui → lipgloss, isatty, termenv
workspace, vscodesettings → peacock, jsonc
runner → color, workspace, vscodesettings
interactive → runner, vscodesettings
cmd/ccws → runner, interactive, tui
```

## Non-goals (don't add these without design discussion)

Peacock favorites, `peacock.remoteColor` / Live Share, multi-root workspaces, VSCode Profiles integration, `.code-workspace` comment preservation on rewrite, uninstall subcommand.
