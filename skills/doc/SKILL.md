---
name: doc
description: Use the doc CLI to validate relative links and GitHub-style heading anchors in Git-hosted Markdown, or to move repository paths while rewriting affected Markdown links. Apply when checking documentation links, validating staged docs before commit, or renaming and reorganizing linked files, directories, and assets.
---

# doc CLI

Use `doc` inside the target Git worktree. Prefer running at the repository root so move arguments and reported paths are unambiguous.

## Choose the command

- Run `doc lint` to check the working-tree documentation set: tracked Markdown plus untracked, non-ignored Markdown. Use this after documentation edits or link-affecting file changes.
- Run `doc lint --staged` for a pre-commit check. It reads documents and targets from the Git index and checks only added, copied, modified, or renamed Markdown inputs. It intentionally does not validate unstaged contents.
- Run `doc mv <source> <destination>` when the user has asked to move or rename a repository path and Markdown links must follow it. It supports files, directories, and non-Markdown assets referenced by Markdown.

Do not use `doc mv` merely to diagnose a problem: it mutates the working tree. Do not stage its changes unless the user also requested staging or committing.

## Moving paths

The source and destination may be absolute or relative to the current directory, but both must remain inside the same Git worktree. The destination parent must already exist. `doc mv` refuses collisions, Git metadata, repository escapes, and moving a directory beneath itself.

The move rewrites affected local inline links, images, and reference definitions, including inbound links to the moved subtree and relative links inside moved Markdown files. It preserves query strings, fragments, titles, surrounding Markdown, and line endings. It does not update ignored Markdown files or links embedded in code spans, code blocks, or raw HTML.

After a successful move:

1. Inspect `git status --short` and `git diff --` to verify the complete working-tree change.
2. Run `doc lint` to catch link issues outside the rewrite guarantee.
3. Report that the move is unstaged unless subsequent authorized work staged it.

## Interpret results

Lint findings use `path:line:column: message` and cover missing relative targets, incorrect path casing, invalid local path encoding or repository escapes, and missing GitHub-style heading anchors. Network URLs, email links, other URI schemes, and root-relative site URLs are outside its validation scope.

- Exit `0`: command succeeded; lint found no issues.
- Exit `1`: lint found issues or a move was safely rejected. Treat lint output as findings, not an execution failure.
- Exit `2`: invalid invocation or a Git/index/internal error. Report the error and resolve the underlying condition before retrying.

If `doc` is unavailable, do not silently replace `doc mv` with a plain filesystem move because that loses the link-maintenance guarantee. Report the missing CLI and use the repository's documented installation method only when installation is within the user's requested scope.
