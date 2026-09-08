# doc

`doc` (documents operational commands) is a small command-line tool for keeping
links in a Git repository's Markdown documentation healthy.

## Install

Go 1.25 or newer is required to build from source.

```sh
go install github.com/Harineko0/doc/cmd/doc@latest
```

For a local checkout:

```sh
go build -o doc ./cmd/doc
```

## Commands

### `doc lint`

```sh
doc lint
doc lint --staged
```

`doc lint` checks every tracked Markdown file and every untracked Markdown file
that is not excluded by `.gitignore`. File extensions `.md` and `.markdown` are
matched case-insensitively. It reports missing relative targets, incorrect path
casing, and missing GitHub-style heading anchors.

`doc lint --staged` is intended for pre-commit hooks. It checks only added,
copied, modified, or renamed Markdown paths in the Git index. Both the document
contents and their targets are read from the index, so partially staged files
are checked exactly as they will be committed. A staged deletion therefore
makes a link to that path invalid. Deleted Markdown documents are not lint
inputs.

Diagnostics have a stable compiler-style format:

```text
guide/start.md:12:18: link target does not exist: guide/missing.md
```

### `doc mv`

```sh
doc mv abc.md foo/abc.md
doc mv guides archive/guides
```

`doc mv` moves a file or directory and updates relative Markdown links across
the repository. It updates links pointing into the moved subtree as well as
relative links inside Markdown files whose own location changed. Inline links,
images, and reference-link definitions are supported. Query strings, fragments,
titles, surrounding Markdown, and line endings are preserved.

The destination parent directory must already exist. The command refuses to
overwrite another path, escape the repository, or move a directory beneath
itself. It edits the working tree only; it does not stage changes. Ignored
Markdown files are outside the link-maintenance guarantee.

## Link rules

Local relative destinations are checked. URL paths are percent-decoded for file
lookup, and required characters are percent-encoded when a moved link is
rewritten. Links in code spans, code blocks, and raw HTML are ignored. Network
URLs, email links, other URI schemes, and `/`-rooted site URLs are not fetched or
validated. Heading fragments use lowercase GitHub-style slugs, preserve Unicode
letters and numbers, and suffix duplicate headings with `-1`, `-2`, and so on.

## Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Successful command; lint found no problems |
| `1` | Lint findings or a safely rejected move |
| `2` | Invalid invocation, Git/index failure, or internal execution error |
