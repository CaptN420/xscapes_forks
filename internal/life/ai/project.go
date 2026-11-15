package ai

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Looking at the project, safely.
//
// This is the part of the companion that is allowed to read source code, so it
// is the part written most defensively:
//
//	READ ONLY. Nothing here opens a file for writing, and nothing runs a
//	command. The only syscalls are Open and ReadDir.
//
//	BOUNDED. A file count, a byte budget and an extension list, all constants
//	in this file.
//
//	EXPLAINED. What it read is carried into the request as facts, and anything
//	a model claims is checked against the filesystem before it is shown.

// Scan limits. Small on purpose: this is a companion glancing at a project, not
// an indexer, and the whole point is that it costs nothing to leave running.
const (
	MaxFiles     = 12
	MaxFileBytes = 24 * 1024
)

// skipDirs are never walked: they are either enormous, generated, or private.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".venv": true, "venv": true, "__pycache__": true,
	".idea": true, ".vscode": true, "assets": true, "notes": true,
}

// keepExt is what a companion can usefully glance at. Source only.
var keepExt = map[string]bool{
	".go": true, ".py": true, ".ts": true, ".tsx": true, ".js": true,
	".rs": true, ".rb": true, ".java": true, ".kt": true, ".c": true,
	".h": true, ".cpp": true, ".cs": true, ".sh": true, ".sql": true,
}

// Fact is one thing the companion noticed in the project, in a form a person can
// check: a path, a line number where one applies, and a short note.
type Fact struct {
	Path string
	Line int
	Note string
}

// String renders a fact the way a citation should read.
func (f Fact) String() string {
	if f.Line > 0 {
		return f.Path + ":" + itoa(f.Line) + " " + f.Note
	}
	if f.Note == "" {
		return f.Path
	}
	return f.Path + " " + f.Note
}

// declRe finds a top-level declaration, which is the cheapest interesting thing
// to say about a file without parsing it.
var declRe = regexp.MustCompile(`^(func|type|class|def|struct|impl|interface|export function|const|var)\s+([A-Za-z_][A-Za-z0-9_]*)`)

// ScanProject reads a bounded, deterministic sample of a project and returns
// what it found. It never writes, never follows symlinks out of the tree, and
// never returns an error for a file it could not read -- a companion does not
// complain about permissions.
func ScanProject(root string, maxFiles int) []Fact {
	if maxFiles <= 0 || maxFiles > MaxFiles {
		maxFiles = MaxFiles
	}
	var facts []Fact
	seen := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if seen >= maxFiles {
			return fs.SkipAll
		}
		if !keepExt[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Size() > MaxFileBytes {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if f, ok := glance(filepath.Join(root, rel), rel); ok {
			facts = append(facts, f)
			seen++
		}
		return nil
	})
	sort.Slice(facts, func(i, j int) bool { return facts[i].Path < facts[j].Path })
	return facts
}

// glance reads one file's head and reports the first declaration it can name.
// Reading is the only thing it does with the file.
func glance(abs, rel string) (Fact, bool) {
	f, err := os.Open(abs)
	if err != nil {
		return Fact{}, false
	}
	defer f.Close()
	buf := make([]byte, 8*1024)
	n, _ := f.Read(buf)
	if n <= 0 {
		return Fact{}, false
	}
	lines := strings.Split(string(buf[:n]), "\n")
	for i, line := range lines {
		if m := declRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return Fact{Path: rel, Line: i + 1, Note: m[1] + " " + m[2]}, true
		}
	}
	return Fact{}, false
}

// CitationOK checks that a claim of the form "path/to/file.go:123" names a file
// that exists under root, and a line that file actually has. A model's answer is
// shown to the user only if this passes, which is the difference between an
// observation and a fabrication.
func CitationOK(ref, root string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	path, line := ref, 0
	if i := strings.LastIndex(ref, ":"); i > 0 {
		if n, ok := atoi(ref[i+1:]); ok {
			path, line = ref[:i], n
		}
	}
	// A path that climbs out of the tree is not a citation.
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return false
	}
	abs := filepath.Join(root, clean)
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || info.Size() > MaxFileBytes {
		return false
	}
	if line <= 0 {
		return true
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return false
	}
	return line <= strings.Count(string(data), "\n")+1
}

// Citation pulls the first path:line out of an answer, if it has one.
var citeRe = regexp.MustCompile(`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+:[0-9]+`)

func Citation(text string) string { return citeRe.FindString(text) }

func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 1<<30 {
			return 0, false
		}
	}
	return n, true
}
