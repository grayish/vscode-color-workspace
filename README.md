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
3. Write `/home/me/code/myproj.code-workspace` (merging peacock keys into any existing file). **If the workspace file already contains peacock keys, ccws skips the write, prints a warning, and just opens it. Pass `--force` to overwrite.**
4. Clean up `peacock.*` keys and the peacock-managed subset of `workbench.colorCustomizations` from `/home/me/code/myproj/.vscode/settings.json`. If the settings file becomes empty it's deleted, along with an empty `.vscode/` directory.
5. Launch `code <workspace-file>`.

## Worktree color family

When you run `ccws` inside a git worktree, it automatically picks a "family" color so sibling worktrees of the same repo look related but distinct (same hue/saturation, lightness shifted by ±1 to ±7%).

- First `ccws` on the main worktree: random color, becomes the family anchor.
- First `ccws` on a linked worktree (main not yet colored): a random anchor is written to the main worktree's `.code-workspace` automatically, and the linked worktree gets a derived color. A warning is printed to stderr.
- If linked worktrees already have colors but main does not, ccws assumes you set them deliberately, prints a warning, and disables family logic for that run.
- Pass `--color` to bypass family logic.

The worktree identity is stable across branch renames and `git worktree move` (it uses the name git assigns under `.git/worktrees/<name>`).

## Safety guards

- **Guard 1 (soft) — existing peacock keys in the workspace file.** ccws prints a warning, opens the workspace as-is, and exits 0. `.vscode/settings.json` is not touched on this path. Pass `--force` to overwrite (this also re-runs cleanup against `.vscode/settings.json`).
- **Guard 2 — non-peacock `workbench.colorCustomizations` would remain in `.vscode/settings.json`.** ccws refuses to proceed and exits with code 2. Remove those keys manually or pass `--force`.

`ccws interactive` shows Guard 1 as a 3-option pre-check (Open existing / Overwrite / Cancel) before the form, and Guard 2 as a confirmation prompt during the run.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | success (including the soft Guard 1 case where ccws opens an existing peacock workspace) |
| 1 | input error (invalid color, missing folder, parse failure) |
| 2 | Guard 2 triggered (non-peacock keys would remain in `.vscode/settings.json` after cleanup) |
| 3 | filesystem error |

> **Behavior change:** prior versions exited with code 2 when the target's workspace file already contained peacock keys. As of this version, ccws prints a warning, opens the existing workspace, and exits 0. Shell scripts that depended on the old exit-2 path for this case must now check stderr for the "workspace already configured" notice or always pass `--force`.

## Non-goals

Peacock favorites, `peacock.remoteColor` / Live Share, multi-root workspaces, per-element lighten/darken adjustments, VSCode Profiles integration, comment preservation in `.code-workspace` (comments are stripped on rewrite).
