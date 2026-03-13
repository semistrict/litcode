package filematch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Match struct {
	AbsPath string
	RelPath string
}

func Index(patterns []string) (map[string]string, error) {
	index := make(map[string]string)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	for _, pattern := range patterns {
		if hasMeta(pattern) {
			matches, err := Collect([]string{pattern}, nil)
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				addIndex(index, match.RelPath, match.AbsPath)
			}
			continue
		}

		info, err := os.Stat(pattern)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", pattern, err)
		}

		absPattern, err := filepath.Abs(pattern)
		if err != nil {
			return nil, err
		}

		if !info.IsDir() {
			rel, err := filepath.Rel(cwd, absPattern)
			if err != nil {
				return nil, err
			}
			addIndex(index, filepath.ToSlash(rel), absPattern)
			addIndex(index, filepath.Base(absPattern), absPattern)
			continue
		}

		if err := filepath.Walk(absPattern, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			relCwd, err := filepath.Rel(cwd, path)
			if err != nil {
				return err
			}
			addIndex(index, filepath.ToSlash(relCwd), path)

			relRoot, err := filepath.Rel(absPattern, path)
			if err != nil {
				return err
			}
			addIndex(index, filepath.ToSlash(relRoot), path)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return index, nil
}

func Collect(patterns []string, keep func(relPath string) bool) ([]Match, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var matches []Match

	add := func(path string) error {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if seen[abs] {
			return nil
		}
		info, err := os.Stat(abs)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(cwd, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if keep != nil && !keep(rel) {
			return nil
		}
		seen[abs] = true
		matches = append(matches, Match{AbsPath: abs, RelPath: rel})
		return nil
	}

	for _, pattern := range patterns {
		if hasMeta(pattern) {
			if err := collectPattern(cwd, pattern, keep, add); err != nil {
				return nil, err
			}
			continue
		}

		info, err := os.Stat(pattern)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", pattern, err)
		}
		if !info.IsDir() {
			if err := add(pattern); err != nil {
				return nil, err
			}
			continue
		}

		if err := filepath.Walk(pattern, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			return add(path)
		}); err != nil {
			return nil, err
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].RelPath < matches[j].RelPath
	})

	return matches, nil
}

func RelPath(absPath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return absPath
	}
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}

func MatchPath(pattern, path string) bool {
	pattern = filepath.ToSlash(filepath.Clean(pattern))
	path = filepath.ToSlash(filepath.Clean(path))
	if pattern == "." {
		return path == "."
	}
	return matchSegments(split(pattern), split(path))
}

func addIndex(index map[string]string, key, absPath string) {
	key = filepath.ToSlash(filepath.Clean(key))
	if key == "." || key == "" {
		return
	}
	if _, ok := index[key]; !ok {
		index[key] = absPath
	}
}

func collectPattern(cwd, pattern string, keep func(relPath string) bool, add func(path string) error) error {
	base := literalPrefixDir(pattern)
	absBase := filepath.Join(cwd, filepath.FromSlash(base))
	info, err := os.Stat(absBase)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		rel := filepath.ToSlash(base)
		if MatchPath(pattern, rel) {
			if keep == nil || keep(rel) {
				return add(absBase)
			}
		}
		return nil
	}

	return filepath.Walk(absBase, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if MatchPath(pattern, rel) {
			if keep == nil || keep(rel) {
				return add(path)
			}
		}
		return nil
	})
}

func hasMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func literalPrefixDir(pattern string) string {
	parts := split(filepath.ToSlash(pattern))
	var prefix []string
	for _, part := range parts {
		if hasMeta(part) {
			break
		}
		prefix = append(prefix, part)
	}
	if len(prefix) == 0 {
		return "."
	}
	return strings.Join(prefix, "/")
}

func split(path string) []string {
	if path == "." || path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func matchSegments(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	part := patternParts[0]
	if part == "**" {
		if matchSegments(patternParts[1:], pathParts) {
			return true
		}
		return len(pathParts) > 0 && matchSegments(patternParts, pathParts[1:])
	}

	if len(pathParts) == 0 {
		return false
	}

	matched, err := filepath.Match(part, pathParts[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(patternParts[1:], pathParts[1:])
}
