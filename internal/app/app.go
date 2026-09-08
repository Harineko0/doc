package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Harineko0/doc/internal/gitstore"
	"github.com/Harineko0/doc/internal/mdparse"
)

type finding struct {
	path    string
	line    int
	column  int
	message string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d:%d: %s", filepath.ToSlash(f.path), f.line, f.column, f.message)
}

type snapshot interface {
	Read(path string) ([]byte, error)
	Kind(path string) (kind string, exact bool)
}

type workingSnapshot struct{ root string }

func (s workingSnapshot) Read(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.root, path))
}

func (s workingSnapshot) Kind(path string) (string, bool) {
	path = filepath.Clean(path)
	if path == "." {
		return "directory", true
	}
	current := s.root
	exact := true
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", false
		}
		actual := ""
		for _, entry := range entries {
			if entry.Name() == component {
				actual = component
				break
			}
			if actual == "" && strings.EqualFold(entry.Name(), component) {
				actual = entry.Name()
			}
		}
		if actual == "" {
			return "", false
		}
		if actual != component {
			exact = false
		}
		current = filepath.Join(current, actual)
	}
	info, err := os.Stat(current)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return "directory", exact
	}
	return "file", exact
}

type indexSnapshot struct{ index *gitstore.Index }

func (s indexSnapshot) Read(path string) ([]byte, error) { return s.index.Read(path) }
func (s indexSnapshot) Kind(path string) (string, bool)  { return s.index.Kind(path) }

func Lint(staged bool) ([]string, error) {
	root, err := gitstore.Root()
	if err != nil {
		return nil, err
	}

	var paths []string
	var view snapshot
	if staged {
		index, err := gitstore.LoadIndex(root)
		if err != nil {
			return nil, err
		}
		paths, err = gitstore.StagedMarkdown(root)
		if err != nil {
			return nil, err
		}
		view = indexSnapshot{index: index}
	} else {
		paths, err = gitstore.WorkingMarkdown(root)
		if err != nil {
			return nil, err
		}
		view = workingSnapshot{root: root}
	}

	cache := map[string]mdparse.Document{}
	var findings []finding
	for _, path := range paths {
		if !staged {
			if _, err := os.Lstat(filepath.Join(root, path)); errors.Is(err, fs.ErrNotExist) {
				continue
			}
		}
		data, err := view.Read(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
		}
		doc := mdparse.Parse(data)
		cache[filepath.Clean(path)] = doc
		for _, link := range doc.Links {
			if !mdparse.IsLocal(link.Destination) {
				continue
			}
			linkPath, err := mdparse.DecodePath(link.Destination)
			if err != nil {
				findings = append(findings, newFinding(path, link, "invalid percent-encoding in link destination"))
				continue
			}
			target, inside := resolveLinkTarget(path, linkPath)
			if !inside {
				findings = append(findings, newFinding(path, link, "local link escapes the repository"))
				continue
			}
			kind, exact := view.Kind(target)
			if kind == "" {
				findings = append(findings, newFinding(path, link, fmt.Sprintf("link target does not exist: %s", filepath.ToSlash(target))))
				continue
			}
			if !exact {
				findings = append(findings, newFinding(path, link, fmt.Sprintf("link target has incorrect path casing: %s", filepath.ToSlash(target))))
				continue
			}
			fragment, hasFragment := mdparse.Fragment(mdparse.DecodeDestination(link.Destination))
			if !hasFragment || fragment == "" || kind != "file" || !gitstore.IsMarkdown(target) {
				continue
			}
			targetDoc, ok := cache[target]
			if !ok {
				contents, err := view.Read(target)
				if err != nil {
					return nil, fmt.Errorf("read link target %s: %w", filepath.ToSlash(target), err)
				}
				targetDoc = mdparse.Parse(contents)
				cache[target] = targetDoc
			}
			if _, ok := targetDoc.Anchors[fragment]; !ok {
				findings = append(findings, newFinding(path, link, fmt.Sprintf("heading anchor does not exist: #%s", fragment)))
			}
		}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].path != findings[j].path {
			return findings[i].path < findings[j].path
		}
		if findings[i].line != findings[j].line {
			return findings[i].line < findings[j].line
		}
		return findings[i].column < findings[j].column
	})
	result := make([]string, len(findings))
	for i, item := range findings {
		result[i] = item.String()
	}
	return result, nil
}

func newFinding(path string, link mdparse.Link, message string) finding {
	return finding{path: path, line: link.Line, column: link.Column, message: message}
}

func resolveRepoPath(base, markdownPath string) (string, bool) {
	markdownPath = filepath.FromSlash(markdownPath)
	target := filepath.Clean(filepath.Join(base, markdownPath))
	if target == ".." || strings.HasPrefix(target, ".."+string(filepath.Separator)) || filepath.IsAbs(target) {
		return target, false
	}
	return target, true
}

func resolveLinkTarget(documentPath, markdownPath string) (string, bool) {
	if markdownPath == "" {
		return filepath.Clean(documentPath), true
	}
	return resolveRepoPath(filepath.Dir(documentPath), markdownPath)
}

type MoveResult struct {
	Source      string
	Destination string
	Updated     int
}

type internalError struct{ err error }

func (e internalError) Error() string { return e.err.Error() }
func (e internalError) Unwrap() error { return e.err }

func IsInternalError(err error) bool {
	var target internalError
	return errors.As(err, &target)
}

func internal(err error) error { return internalError{err: err} }

type plannedFile struct {
	oldPath  string
	newPath  string
	original []byte
	updated  []byte
	mode     fs.FileMode
}

var writeUpdatedFile = atomicWrite

func Move(sourceArg, destinationArg string) (MoveResult, error) {
	root, err := gitstore.Root()
	if err != nil {
		return MoveResult{}, internal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return MoveResult{}, internal(err)
	}
	sourceAbs, sourceRel, err := repoArgument(root, cwd, sourceArg)
	if err != nil {
		return MoveResult{}, fmt.Errorf("source: %w", err)
	}
	destinationAbs, destinationRel, err := repoArgument(root, cwd, destinationArg)
	if err != nil {
		return MoveResult{}, fmt.Errorf("destination: %w", err)
	}
	if sourceRel == destinationRel {
		return MoveResult{}, fmt.Errorf("source and destination are the same path")
	}
	if isGitMetadata(sourceRel) || isGitMetadata(destinationRel) {
		return MoveResult{}, fmt.Errorf("cannot move Git metadata")
	}
	sourceInfo, err := os.Lstat(sourceAbs)
	if err != nil {
		return MoveResult{}, fmt.Errorf("source: %w", err)
	}
	if exists, exact := pathEntryCase(root, sourceRel); !exists || !exact {
		return MoveResult{}, fmt.Errorf("source path has incorrect casing")
	}
	if err := ensureParentInsideRepository(root, sourceAbs); err != nil {
		return MoveResult{}, fmt.Errorf("source: %w", err)
	}
	parentInfo, err := os.Stat(filepath.Dir(destinationAbs))
	if err != nil || !parentInfo.IsDir() {
		return MoveResult{}, fmt.Errorf("destination parent directory does not exist")
	}
	if kind, exact := (workingSnapshot{root: root}).Kind(filepath.Dir(destinationRel)); kind != "directory" || !exact {
		return MoveResult{}, fmt.Errorf("destination parent path has incorrect casing")
	}
	if err := ensureParentInsideRepository(root, destinationAbs); err != nil {
		return MoveResult{}, fmt.Errorf("destination: %w", err)
	}
	if sourceInfo.IsDir() && isSameOrChild(destinationRel, sourceRel) {
		return MoveResult{}, fmt.Errorf("cannot move a directory into itself")
	}

	caseOnly := false
	if destinationInfo, statErr := os.Lstat(destinationAbs); statErr == nil {
		destinationEntryExists, entryErr := hasExactDirectoryEntry(filepath.Dir(destinationAbs), filepath.Base(destinationAbs))
		if entryErr != nil {
			return MoveResult{}, internal(fmt.Errorf("inspect destination directory: %w", entryErr))
		}
		caseOnly = strings.EqualFold(sourceAbs, destinationAbs) && os.SameFile(sourceInfo, destinationInfo) && !destinationEntryExists
		if !caseOnly {
			return MoveResult{}, fmt.Errorf("destination already exists")
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return MoveResult{}, fmt.Errorf("inspect destination: %w", statErr)
	}

	markdownPaths, err := gitstore.WorkingMarkdown(root)
	if err != nil {
		return MoveResult{}, internal(err)
	}
	files := make([]plannedFile, 0, len(markdownPaths))
	for _, oldPath := range markdownPaths {
		absolute := filepath.Join(root, oldPath)
		info, err := os.Lstat(absolute)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return MoveResult{}, internal(fmt.Errorf("inspect %s: %w", filepath.ToSlash(oldPath), err))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		original, err := os.ReadFile(absolute)
		if err != nil {
			return MoveResult{}, internal(fmt.Errorf("read %s: %w", filepath.ToSlash(oldPath), err))
		}
		newPath := remapPath(oldPath, sourceRel, destinationRel)
		doc := mdparse.Parse(original)
		var edits []mdparse.Edit
		for _, link := range doc.Links {
			if !mdparse.IsLocal(link.Destination) {
				continue
			}
			decoded := mdparse.DecodeDestination(link.Destination)
			linkPath, decodeErr := mdparse.DecodePath(link.Destination)
			if decodeErr != nil {
				continue
			}
			oldTarget, inside := resolveLinkTarget(oldPath, linkPath)
			if !inside {
				continue
			}
			newTarget := remapPath(oldTarget, sourceRel, destinationRel)
			if oldPath == newPath && oldTarget == newTarget {
				continue
			}
			pathPart, suffix := mdparse.SplitDestination(decoded)
			newLinkPath := ""
			if pathPart != "" {
				relative, err := filepath.Rel(filepath.Dir(newPath), newTarget)
				if err != nil {
					return MoveResult{}, internal(fmt.Errorf("calculate link from %s: %w", filepath.ToSlash(newPath), err))
				}
				newLinkPath = mdparse.EncodePath(filepath.ToSlash(relative))
				if strings.HasSuffix(pathPart, "/") && !strings.HasSuffix(newLinkPath, "/") {
					newLinkPath += "/"
				}
			}
			newDestination := newLinkPath + suffix
			if newDestination != link.Destination {
				edits = append(edits, mdparse.Edit{Start: link.Start, End: link.End, Text: newDestination})
			}
		}
		updated := mdparse.Apply(original, edits)
		files = append(files, plannedFile{oldPath: oldPath, newPath: newPath, original: original, updated: updated, mode: info.Mode()})
	}

	if err := renameForMove(sourceAbs, destinationAbs, caseOnly); err != nil {
		return MoveResult{}, internal(err)
	}
	written := make([]plannedFile, 0, len(files))
	for _, file := range files {
		if string(file.original) == string(file.updated) {
			continue
		}
		path := filepath.Join(root, file.newPath)
		if err := writeUpdatedFile(path, file.updated, file.mode); err != nil {
			rollbackErr := rollbackMove(root, sourceAbs, destinationAbs, caseOnly, written)
			if rollbackErr != nil {
				return MoveResult{}, internal(fmt.Errorf("update %s: %w (rollback failed: %v)", filepath.ToSlash(file.newPath), err, rollbackErr))
			}
			return MoveResult{}, internal(fmt.Errorf("update %s: %w; move was rolled back", filepath.ToSlash(file.newPath), err))
		}
		written = append(written, file)
	}

	return MoveResult{Source: filepath.ToSlash(sourceRel), Destination: filepath.ToSlash(destinationRel), Updated: len(written)}, nil
}

func hasExactDirectoryEntry(directory, name string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name() == name {
			return true, nil
		}
	}
	return false, nil
}

func pathEntryCase(root, path string) (exists, exact bool) {
	path = filepath.Clean(path)
	if path == "." {
		return true, true
	}
	current := root
	exact = true
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		entries, err := os.ReadDir(current)
		if err != nil {
			return false, false
		}
		actual := ""
		for _, entry := range entries {
			if entry.Name() == component {
				actual = component
				break
			}
			if actual == "" && strings.EqualFold(entry.Name(), component) {
				actual = entry.Name()
			}
		}
		if actual == "" {
			return false, false
		}
		if actual != component {
			exact = false
		}
		current = filepath.Join(current, actual)
	}
	return true, exact
}

func repoArgument(root, cwd, value string) (absolute, relative string, err error) {
	if filepath.IsAbs(value) {
		absolute = filepath.Clean(value)
	} else {
		absolute = filepath.Clean(filepath.Join(cwd, value))
	}
	if realParent, resolveErr := filepath.EvalSymlinks(filepath.Dir(absolute)); resolveErr == nil {
		absolute = filepath.Join(realParent, filepath.Base(absolute))
	}
	relative, err = filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("path must be inside repository %s", root)
	}
	return absolute, filepath.Clean(relative), nil
}

func isGitMetadata(path string) bool {
	path = filepath.Clean(path)
	first := path
	if separator := strings.IndexRune(path, filepath.Separator); separator >= 0 {
		first = path[:separator]
	}
	return strings.EqualFold(first, ".git")
}

func ensureParentInsideRepository(root, path string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(realRoot, realParent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path resolves outside the repository")
	}
	return nil
}

func remapPath(path, source, destination string) string {
	path = filepath.Clean(path)
	if path == source {
		return destination
	}
	if isSameOrChild(path, source) {
		remainder, _ := filepath.Rel(source, path)
		return filepath.Join(destination, remainder)
	}
	return path
}

func isSameOrChild(path, parent string) bool {
	if path == parent {
		return true
	}
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func renameForMove(source, destination string, caseOnly bool) error {
	if !caseOnly {
		if err := os.Rename(source, destination); err != nil {
			return fmt.Errorf("move %s to %s: %w", source, destination, err)
		}
		return nil
	}
	temporary, err := unusedSibling(source)
	if err != nil {
		return err
	}
	if err := os.Rename(source, temporary); err != nil {
		return fmt.Errorf("prepare case-only rename: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Rename(temporary, source)
		return fmt.Errorf("complete case-only rename: %w", err)
	}
	return nil
}

func unusedSibling(path string) (string, error) {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".doc-mv-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

func atomicWrite(path string, contents []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".doc-write-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(mode.Perm()); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(contents); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func rollbackMove(root, sourceAbs, destinationAbs string, caseOnly bool, written []plannedFile) error {
	var rollbackErrors []string
	for i := len(written) - 1; i >= 0; i-- {
		file := written[i]
		current := filepath.Join(root, file.newPath)
		if err := atomicWrite(current, file.original, file.mode); err != nil {
			rollbackErrors = append(rollbackErrors, err.Error())
		}
	}
	if err := renameForMove(destinationAbs, sourceAbs, caseOnly); err != nil {
		rollbackErrors = append(rollbackErrors, err.Error())
	}
	if len(rollbackErrors) != 0 {
		return errors.New(strings.Join(rollbackErrors, "; "))
	}
	return nil
}
