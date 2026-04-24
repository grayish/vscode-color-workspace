# vscode-color-workspace (`ccws`)

`ccws` generates a `.code-workspace` file with [Peacock](https://github.com/johnpapa/vscode-peacock)-equivalent colors in the parent directory of your project, so your per-project color setup never lands in the shared `.vscode/settings.json`.

## Why

Peacock stores its colors in `.vscode/settings.json` — the same file teams use for shared project settings — so your personal color preferences end up in Git. `ccws` writes the colors to `<parent>/<folder>.code-workspace` instead and opens the workspace with `code`. VSCode's workspace file scope is effectively private.

## Install

```bash
go install github.com/sang-bin/vscode-color-workspace/cmd/ccws@latest
```

## Usage

```bash
# Random color, auto-open
ccws

# Specific color by hex or CSS name
ccws --color '#5a3b8c'
ccws --color rebeccapurple

# Target a specific directory
ccws --color red /path/to/myproj

# Walk through all options
ccws interactive /path/to/myproj

# Overwrite existing peacock color settings
ccws --color '#ff0000' --force

# Skip opening (CI / scripts)
ccws --color random --no-open
```

Running `ccws` in `/home/me/code/myproj` will:

1. Resolve the color (explicit `--color` > `peacock.color` from `.vscode/settings.json` > random).
2. Generate the Peacock palette (activityBar / statusBar / titleBar by default).
3. Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file).
4. Clean up `peacock.*` keys and the peacock-managed subset of `workbench.colorCustomizations` from `/home/me/code/myproj/.vscode/settings.json`. If the settings file becomes empty it's deleted, along with an empty `.vscode/` directory.
5. Launch `code <workspace-file>`.

## Safety guards

`ccws` refuses to proceed and exits with code `2` in two situations:

- **Guard 1 — existing peacock keys in the workspace file.** Pass `--force` to overwrite.
- **Guard 2 — non-peacock `workbench.colorCustomizations` would remain in `.vscode/settings.json`.** Remove those keys manually or pass `--force`.

`ccws interactive` shows the same guards as explicit confirmation prompts.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success |
| 1 | input error (invalid color, missing folder, parse failure) |
| 2 | safety guard triggered |
| 3 | filesystem error |

## Non-goals

Peacock favorites, `peacock.remoteColor` / Live Share, multi-root workspaces, per-element lighten/darken adjustments, VSCode Profiles integration, comment preservation in `.code-workspace` (comments are stripped on rewrite).
