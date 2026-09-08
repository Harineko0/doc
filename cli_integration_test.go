package doc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltCLI(t *testing.T) {
	projectRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "doc")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/doc")
	build.Dir = projectRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	writeCLIFile(t, repo, "README.md", "[page](page.md#part)\n")

	command := exec.Command(binary, "lint")
	command.Dir = repo
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !strings.Contains(string(output), "link target does not exist") || !asExitError(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("broken lint: exit=%v output=%s", err, output)
	}

	writeCLIFile(t, repo, "page.md", "# Part\n")
	command = exec.Command(binary, "lint")
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("clean lint: %v\n%s", err, output)
	}
	if err := os.Mkdir(filepath.Join(repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	command = exec.Command(binary, "mv", "page.md", "docs/page.md")
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("move: %v\n%s", err, output)
	}
	contents, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "[page](docs/page.md#part)\n" {
		t.Fatalf("rewritten README = %q", contents)
	}

	command = exec.Command(binary, "lint", "--unknown")
	command.Dir = repo
	if output, err := command.CombinedOutput(); !asExitError(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("invalid invocation: exit=%v output=%s", err, output)
	}
}

func asExitError(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	value, ok := err.(*exec.ExitError)
	if ok {
		*target = value
	}
	return ok
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func writeCLIFile(t *testing.T, repo, path, contents string) {
	t.Helper()
	absolute := filepath.Join(repo, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
