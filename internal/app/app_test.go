package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintWorkingTree(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "# Home\n\n[good](docs/guide.md#start-here)\n[case](docs/Guide.md)\n[missing](missing.md)\n[external](https://example.com)\n")
	write(t, repo, "docs/guide.md", "# Start Here\n")
	write(t, repo, "UPPER.MARKDOWN", "[upper](upper-missing.md)\n")
	write(t, repo, "ignored.md", "[ignored](missing.md)\n")
	write(t, repo, ".gitignore", "ignored.md\n")

	withCWD(t, repo, func() {
		findings, err := Lint(false)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(findings, "\n")
		if !strings.Contains(joined, "incorrect path casing: docs/Guide.md") {
			t.Errorf("missing casing diagnostic:\n%s", joined)
		}
		if !strings.Contains(joined, "does not exist: missing.md") {
			t.Errorf("missing target diagnostic:\n%s", joined)
		}
		if !strings.Contains(joined, "UPPER.MARKDOWN") {
			t.Errorf("uppercase Markdown extension was not linted:\n%s", joined)
		}
		if strings.Contains(joined, "ignored.md") || strings.Contains(joined, "example.com") {
			t.Errorf("unexpected diagnostic:\n%s", joined)
		}
	})
}

func TestLintSameDocumentAnchor(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "# Existing\n\n[good](#existing)\n[bad](#missing)\n[self]()\n")
	withCWD(t, repo, func() {
		findings, err := Lint(false)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || !strings.Contains(findings[0], "#missing") {
			t.Fatalf("findings: %v", findings)
		}
	})
}

func TestLintStagedUsesIndexSnapshot(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "target.md", "# Committed Heading\n")
	write(t, repo, "README.md", "[target](target.md#committed-heading)\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")

	write(t, repo, "README.md", "[target](target.md#staged-heading)\n")
	write(t, repo, "target.md", "# Staged Heading\n")
	git(t, repo, "add", "README.md", "target.md")
	write(t, repo, "target.md", "# Working Tree Only\n")

	withCWD(t, repo, func() {
		findings, err := Lint(true)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 0 {
			t.Fatalf("staged lint findings: %v", findings)
		}
		working, err := Lint(false)
		if err != nil {
			t.Fatal(err)
		}
		if len(working) != 1 || !strings.Contains(working[0], "heading anchor does not exist") {
			t.Fatalf("working lint findings: %v", working)
		}
	})
}

func TestLintStagedInitialCommitAndDeletion(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "[new](new.md)\n")
	write(t, repo, "new.md", "# New\n")
	git(t, repo, "add", ".")
	withCWD(t, repo, func() {
		findings, err := Lint(true)
		if err != nil || len(findings) != 0 {
			t.Fatalf("initial staged lint = %v, %v", findings, err)
		}
	})
	git(t, repo, "commit", "-m", "initial")
	git(t, repo, "rm", "new.md")
	write(t, repo, "README.md", "[new](new.md)\n\nchanged\n")
	git(t, repo, "add", "README.md")
	withCWD(t, repo, func() {
		findings, err := Lint(true)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 || !strings.Contains(findings[0], "does not exist") {
			t.Fatalf("deletion findings: %v", findings)
		}
	})
}

func TestLintStagedRename(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "old.md", "[target](target.md)\n")
	write(t, repo, "target.md", "# Target\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "initial")
	git(t, repo, "mv", "old.md", "renamed.MARKDOWN")
	withCWD(t, repo, func() {
		findings, err := Lint(true)
		if err != nil || len(findings) != 0 {
			t.Fatalf("renamed staged lint = %v, %v", findings, err)
		}
	})
}

func TestLintStagedRejectsUnmergedIndex(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "base\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	git(t, repo, "checkout", "-q", "-b", "other")
	write(t, repo, "README.md", "other\n")
	git(t, repo, "commit", "-am", "other")
	git(t, repo, "checkout", "-q", "main")
	write(t, repo, "README.md", "main\n")
	git(t, repo, "commit", "-am", "main")
	command := exec.Command("git", "-C", repo, "merge", "other")
	if err := command.Run(); err == nil {
		t.Fatal("merge unexpectedly succeeded")
	}
	withCWD(t, repo, func() {
		if _, err := Lint(true); err == nil || !strings.Contains(err.Error(), "unmerged") {
			t.Fatalf("unmerged error = %v", err)
		}
	})
}

func TestMoveFileUpdatesInboundAndOutboundLinks(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "[page](docs/page.md#part)\n")
	write(t, repo, "docs/page.md", "# Part\n\n[home](../README.md)\n![asset](../assets/a%20b.png?raw=1#preview)\n")
	write(t, repo, "assets/a b.png", "data")
	if err := os.MkdirAll(filepath.Join(repo, "archive/deep"), 0o755); err != nil {
		t.Fatal(err)
	}

	withCWD(t, repo, func() {
		result, err := Move("docs/page.md", "archive/deep/page.md")
		if err != nil {
			t.Fatal(err)
		}
		if result.Updated != 2 {
			t.Fatalf("updated = %d, want 2", result.Updated)
		}
	})
	assertContents(t, repo, "README.md", "[page](archive/deep/page.md#part)\n")
	assertContents(t, repo, "archive/deep/page.md", "# Part\n\n[home](../../README.md)\n![asset](../../assets/a%20b.png?raw=1#preview)\n")
	if _, err := os.Stat(filepath.Join(repo, "docs/page.md")); !os.IsNotExist(err) {
		t.Fatalf("old path still exists: %v", err)
	}
}

func TestMoveDirectoryMaintainsInternalAndExternalLinks(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "index.md", "[a](guide/a.md)\n")
	write(t, repo, "guide/a.md", "[b](sub/b.md)\n[index](../index.md)\n")
	write(t, repo, "guide/sub/b.md", "[a](../a.md)\n")
	if err := os.MkdirAll(filepath.Join(repo, "archive"), 0o755); err != nil {
		t.Fatal(err)
	}

	withCWD(t, repo, func() {
		if _, err := Move("guide", "archive/guide"); err != nil {
			t.Fatal(err)
		}
	})
	assertContents(t, repo, "index.md", "[a](archive/guide/a.md)\n")
	assertContents(t, repo, "archive/guide/a.md", "[b](sub/b.md)\n[index](../../index.md)\n")
	assertContents(t, repo, "archive/guide/sub/b.md", "[a](../a.md)\n")
}

func TestMoveNonMarkdownAsset(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "![image](assets/image.png)\n")
	write(t, repo, "assets/image.png", "image")
	if err := os.MkdirAll(filepath.Join(repo, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	withCWD(t, repo, func() {
		if _, err := Move("assets/image.png", "images/image.png"); err != nil {
			t.Fatal(err)
		}
	})
	assertContents(t, repo, "README.md", "![image](images/image.png)\n")
}

func TestMoveRejectsCollisionAndMissingParent(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "a.md", "# A\n")
	write(t, repo, "b.md", "# B\n")
	withCWD(t, repo, func() {
		if _, err := Move("a.md", "b.md"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("collision error = %v", err)
		}
		if _, err := Move("a.md", "missing/a.md"); err == nil || !strings.Contains(err.Error(), "parent") {
			t.Fatalf("parent error = %v", err)
		}
		if _, err := Move(".git/config", "config"); err == nil || !strings.Contains(err.Error(), "Git metadata") {
			t.Fatalf("Git metadata error = %v", err)
		}
	})
	assertContents(t, repo, "a.md", "# A\n")
}

func TestMoveRollsBackAfterWriteFailure(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "[page](docs/page.md)\n")
	write(t, repo, "docs/page.md", "[home](../README.md)\n")
	if err := os.MkdirAll(filepath.Join(repo, "archive/deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	originalWriter := writeUpdatedFile
	calls := 0
	writeUpdatedFile = func(path string, contents []byte, mode os.FileMode) error {
		calls++
		if calls == 2 {
			return os.ErrPermission
		}
		return atomicWrite(path, contents, mode)
	}
	t.Cleanup(func() { writeUpdatedFile = originalWriter })

	withCWD(t, repo, func() {
		if _, err := Move("docs/page.md", "archive/deep/page.md"); err == nil || !strings.Contains(err.Error(), "rolled back") {
			t.Fatalf("move error = %v", err)
		}
	})
	assertContents(t, repo, "README.md", "[page](docs/page.md)\n")
	assertContents(t, repo, "docs/page.md", "[home](../README.md)\n")
	if _, err := os.Stat(filepath.Join(repo, "archive/deep/page.md")); !os.IsNotExist(err) {
		t.Fatalf("destination remains after rollback: %v", err)
	}
}

func TestMoveUpdatesReferenceDefinition(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "See [the page][page].\n\n[page]: <page.md#part> \"Title\"\n")
	write(t, repo, "page.md", "# Part\n")
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	withCWD(t, repo, func() {
		if _, err := Move("page.md", "docs/page.md"); err != nil {
			t.Fatal(err)
		}
	})
	assertContents(t, repo, "README.md", "See [the page][page].\n\n[page]: <docs/page.md#part> \"Title\"\n")
}

func TestMoveSupportsCaseOnlyRename(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "[page](page.md)\n")
	write(t, repo, "page.md", "# Page\n")
	withCWD(t, repo, func() {
		if _, err := Move("page.md", "Page.md"); err != nil {
			t.Fatal(err)
		}
	})
	assertContents(t, repo, "README.md", "[page](Page.md)\n")
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name() == "Page.md" {
			found = true
		}
		if entry.Name() == "page.md" {
			t.Fatalf("old casing remains in directory entry")
		}
	}
	if !found {
		t.Fatal("case-renamed path not found")
	}
}

func TestMoveDoesNotOverwriteDistinctHardLink(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "page.md", "# Page\n")
	if err := os.Link(filepath.Join(repo, "page.md"), filepath.Join(repo, "Page.md")); err != nil {
		t.Skipf("filesystem does not support case-distinct hard links: %v", err)
	}
	withCWD(t, repo, func() {
		if _, err := Move("page.md", "Page.md"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("hard-link collision error = %v", err)
		}
	})
}

func TestMoveBrokenSymlink(t *testing.T) {
	repo := newRepo(t)
	if err := os.Symlink("missing.md", filepath.Join(repo, "link.md")); err != nil {
		t.Skipf("filesystem does not support symlinks: %v", err)
	}
	withCWD(t, repo, func() {
		if _, err := Move("link.md", "renamed.md"); err != nil {
			t.Fatal(err)
		}
	})
	target, err := os.Readlink(filepath.Join(repo, "renamed.md"))
	if err != nil || target != "missing.md" {
		t.Fatalf("moved symlink = %q, %v", target, err)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "user.email", "doc@example.test")
	git(t, repo, "config", "user.name", "doc tests")
	return repo
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func write(t *testing.T, repo, path, contents string) {
	t.Helper()
	absolute := filepath.Join(repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContents(t *testing.T, repo, path, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func withCWD(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	fn()
	if err := os.Chdir(old); err != nil {
		t.Fatal(err)
	}
}
