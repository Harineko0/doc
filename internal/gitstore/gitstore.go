package gitstore

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Entry struct {
	Mode string
	Hash string
}

type Index struct {
	Root    string
	Entries map[string]Entry
	gitDirs map[string]struct{}
}

func Root() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a Git worktree")
	}
	root := filepath.Clean(strings.TrimSpace(string(out)))
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	return root, nil
}

func WorkingMarkdown(root string) ([]string, error) {
	out, err := git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range splitNUL(out) {
		if IsMarkdown(path) {
			paths = append(paths, filepath.FromSlash(path))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func StagedMarkdown(root string) ([]string, error) {
	if out, err := git(root, "ls-files", "-u", "-z"); err != nil {
		return nil, err
	} else if len(out) != 0 {
		return nil, fmt.Errorf("Git index contains unmerged entries")
	}
	out, err := git(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, path := range splitNUL(out) {
		if IsMarkdown(path) {
			paths = append(paths, filepath.FromSlash(path))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func LoadIndex(root string) (*Index, error) {
	out, err := git(root, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, err
	}
	idx := &Index{Root: root, Entries: map[string]Entry{}, gitDirs: map[string]struct{}{}}
	for _, record := range splitNUL(out) {
		tab := strings.IndexByte(record, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("malformed Git index record")
		}
		fields := strings.Fields(record[:tab])
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed Git index metadata")
		}
		stage, err := strconv.Atoi(fields[2])
		if err != nil || stage != 0 {
			return nil, fmt.Errorf("Git index contains unmerged entries")
		}
		path := filepath.Clean(filepath.FromSlash(record[tab+1:]))
		idx.Entries[path] = Entry{Mode: fields[0], Hash: fields[1]}
		for dir := filepath.Dir(path); dir != "."; dir = filepath.Dir(dir) {
			idx.gitDirs[dir] = struct{}{}
		}
	}
	return idx, nil
}

func (i *Index) Read(path string) ([]byte, error) {
	entry, ok := i.Entries[filepath.Clean(path)]
	if !ok {
		return nil, fmt.Errorf("%s is not present in the Git index", filepath.ToSlash(path))
	}
	out, err := git(i.Root, "cat-file", "blob", entry.Hash)
	if err != nil {
		return nil, fmt.Errorf("read index blob %s: %w", filepath.ToSlash(path), err)
	}
	return out, nil
}

func (i *Index) Kind(path string) (kind string, exact bool) {
	path = filepath.Clean(path)
	if path == "." {
		return "directory", true
	}
	if _, ok := i.Entries[path]; ok {
		return "file", true
	}
	if _, ok := i.gitDirs[path]; ok {
		return "directory", true
	}
	for candidate := range i.Entries {
		if strings.EqualFold(candidate, path) {
			return "file", false
		}
	}
	for candidate := range i.gitDirs {
		if strings.EqualFold(candidate, path) {
			return "directory", false
		}
	}
	return "", false
}

func IsMarkdown(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".md") || strings.EqualFold(ext, ".markdown")
}

func git(root string, args ...string) ([]byte, error) {
	all := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", all...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}
	return out, nil
}

func splitNUL(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			result = append(result, string(part))
		}
	}
	return result
}
