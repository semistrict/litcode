package checker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/semistrict/litcode/internal/comments"
	"github.com/semistrict/litcode/internal/expanddocs"
	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/markdown"
)

// Check validates all markdown code blocks in DocsDirs against source files
// found in SourceDirs.
func Check(cfg Config) (*Result, error) {
	docMatches, err := filematch.Collect(cfg.DocsDirs, func(relPath string) bool {
		return strings.HasSuffix(relPath, ".md")
	})
	if err != nil {
		return nil, fmt.Errorf("collecting docs: %w", err)
	}
	mdFiles := make([]string, 0, len(docMatches))
	for _, match := range docMatches {
		mdFiles = append(mdFiles, match.AbsPath)
	}

	sourceIndex, err := filematch.Index(cfg.SourceDirs)
	if err != nil {
		return nil, fmt.Errorf("collecting source files: %w", err)
	}

	// Build exclude lists. User-supplied excludes skip both validation and
	// missing coverage. Default excludes only suppress missing coverage for
	// test code; fixtures and vendored code are skipped entirely.
	validationExcludes := append(DefaultValidationExclude, cfg.Exclude...)
	coverageExcludes := append(DefaultExclude, cfg.Exclude...)

	// Parse all code blocks from markdown files.
	var allBlocks []markdown.CodeBlock
	for _, mf := range mdFiles {
		blocks, err := markdown.ParseFile(mf)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", mf, err)
		}
		allBlocks = append(allBlocks, blocks...)
	}

	// coverage[absPath] = set of 1-based line numbers covered by valid blocks.
	coverage := make(map[string]map[int]bool)
	// invalidCoverage tracks lines referenced by INVALID blocks (with lines=)
	// so they can be suppressed from Missing output.
	invalidCoverage := make(map[string]map[int]bool)
	// referencedFiles tracks absolute paths of source files referenced by any code block.
	referencedFiles := make(map[string]bool)

	// Caches to avoid redundant work across blocks referencing the same file.
	resolveCache := make(map[string]string)         // block.File -> absPath (empty if not found)
	linesCache := make(map[string][]string)         // absPath -> raw lines
	normCache := make(map[string][]string)          // absPath -> normalized lines
	skippableCache := make(map[string]map[int]bool) // absPath -> skippable line set

	// pendingFixes collects fixes to apply after all blocks are validated.
	// Applying them during the loop would shift line numbers for subsequent blocks.
	type pendingFix struct {
		docFile     string
		docLine     int
		oldContent  []string
		newContent  []string
		newStart    int
		newEnd      int
		reason      string
		removeLines bool // strip lines= annotation (content is unique)
	}
	var fixes []pendingFix

	var result Result

	for _, block := range allBlocks {
		// Resolve file path with cache.
		srcPath, found := func() (string, bool) {
			if cached, ok := resolveCache[block.File]; ok {
				return cached, cached != ""
			}
			p, ok := resolveSourceFile(block.File, sourceIndex)
			if ok {
				resolveCache[block.File] = p
			} else {
				resolveCache[block.File] = ""
			}
			return p, ok
		}()
		if !found {
			result.Invalid = append(result.Invalid, InvalidBlock{
				DocFile:    block.DocFile,
				DocLine:    block.DocLine,
				SourceFile: block.File,
				StartLine:  block.StartLine,
				EndLine:    block.EndLine,
				Reason:     "file not found in any source directory",
			})
			continue
		}

		if isExcluded(srcPath, cfg.SourceDirs, validationExcludes) {
			continue
		}

		if !isExcluded(srcPath, cfg.SourceDirs, coverageExcludes) {
			referencedFiles[srcPath] = true
		}

		// Read source lines with cache.
		srcLines, ok := linesCache[srcPath]
		if !ok {
			raw, err := os.ReadFile(srcPath)
			if err != nil {
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					StartLine:  block.StartLine,
					EndLine:    block.EndLine,
					Reason:     fmt.Sprintf("cannot read source file: %v", err),
				})
				continue
			}
			srcLines = strings.Split(string(raw), "\n")
			linesCache[srcPath] = srcLines
			normCache[srcPath] = NormalizeLines(srcLines)
			skippableCache[srcPath] = comments.SkippableLines(filepath.Base(srcPath), raw)
		}

		normSrc := normCache[srcPath]

		if strings.EqualFold(block.Language, "mermaid") {
			coveredLines, err := mermaidCoveredLines(block, srcPath, len(srcLines))
			if err != nil {
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					StartLine:  block.StartLine,
					EndLine:    block.EndLine,
					Reason:     err.Error(),
				})
				continue
			}
			if coverage[srcPath] == nil {
				coverage[srcPath] = make(map[int]bool)
			}
			for _, lineNum := range coveredLines {
				coverage[srcPath][lineNum] = true
			}
			continue
		}

		// No line numbers specified — find content by matching.
		// Try full content first, then retry with doc-only comments stripped.
		if block.StartLine == 0 {
			if expanded, abbreviated, err := expanddocs.ExpandAbbreviatedBlock(block, srcLines); abbreviated {
				if err != nil {
					result.Invalid = append(result.Invalid, InvalidBlock{
						DocFile:    block.DocFile,
						DocLine:    block.DocLine,
						SourceFile: block.File,
						Reason:     err.Error(),
						Diff:       allDeleted(block.Content),
					})
					continue
				}
				matches := findAllContent(expanded, srcLines, normSrc)
				if len(matches) != 1 {
					result.Invalid = append(result.Invalid, InvalidBlock{
						DocFile:    block.DocFile,
						DocLine:    block.DocLine,
						SourceFile: block.File,
						Reason:     "expanded abbreviated block was not uniquely located in source file",
						Diff:       allDeleted(block.Content),
					})
					continue
				}
				start := matches[0] + 1
				end := start + len(expanded) - 1
				if coverage[srcPath] == nil {
					coverage[srcPath] = make(map[int]bool)
				}
				for l := start; l <= end; l++ {
					coverage[srcPath][l] = true
				}
				continue
			}

			content := block.Content
			matches := findAllContent(content, srcLines, normSrc)
			if len(matches) == 0 {
				stripped := stripCommentOnlyLines(content)
				if len(stripped) < len(content) {
					content = stripped
					matches = findAllContent(content, srcLines, normSrc)
				}
			}
			if len(matches) == 1 {
				start := matches[0] + 1
				end := start + len(content) - 1
				if coverage[srcPath] == nil {
					coverage[srcPath] = make(map[int]bool)
				}
				for l := start; l <= end; l++ {
					coverage[srcPath][l] = true
				}
			} else if len(matches) == 0 {
				// Fall through to fuzzy matching, similar to the with-lines path.
				if bestStart, bestEnd, sim := findBestMatchAnywhere(content, normSrc); sim >= 0.70 {
					matchedLines := srcLines[bestStart : bestEnd+1]
					newStart := bestStart + 1
					newEnd := bestEnd + 1
					if cfg.Fix {
						unique := len(findAllContent(matchedLines, srcLines, normSrc)) == 1
						fixes = append(fixes, pendingFix{
							docFile: block.DocFile, docLine: block.DocLine,
							oldContent: block.Content, newContent: matchedLines,
							newStart: newStart, newEnd: newEnd,
							removeLines: unique,
							reason:      fmt.Sprintf("minor edit (%.0f%% similar): content now at lines %d-%d", sim*100, newStart, newEnd),
						})
					} else {
						result.Invalid = append(result.Invalid, InvalidBlock{
							DocFile:    block.DocFile,
							DocLine:    block.DocLine,
							SourceFile: block.File,
							StartLine:  newStart,
							EndLine:    newEnd,
							Reason: fmt.Sprintf("minor edit (%.0f%% similar), content now at lines %d-%d",
								sim*100, newStart, newEnd),
							Diff:    lineDiff(block.Content, matchedLines),
							Fixable: true,
							FixKind: "edit",
						})
					}
					if coverage[srcPath] == nil {
						coverage[srcPath] = make(map[int]bool)
					}
					for l := newStart; l <= newEnd; l++ {
						coverage[srcPath][l] = true
					}
				} else if sim > 0 {
					// Not similar enough to fix, but show comparative diff.
					matchedLines := srcLines[bestStart : bestEnd+1]
					result.Invalid = append(result.Invalid, InvalidBlock{
						DocFile:    block.DocFile,
						DocLine:    block.DocLine,
						SourceFile: block.File,
						Reason:     fmt.Sprintf("content not found in source file (best match %.0f%% similar)", sim*100),
						Diff:       lineDiff(block.Content, matchedLines),
					})
				} else {
					result.Invalid = append(result.Invalid, InvalidBlock{
						DocFile:    block.DocFile,
						DocLine:    block.DocLine,
						SourceFile: block.File,
						Reason:     "content not found in source file",
						Diff:       allDeleted(block.Content),
					})
				}
			} else {
				locs := make([]string, len(matches))
				for i, m := range matches {
					locs[i] = fmt.Sprintf("%d", m+1)
				}
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					Reason: fmt.Sprintf("ambiguous: content found at %d locations (lines %s), add lines= to disambiguate",
						len(matches), strings.Join(locs, ", ")),
				})
			}
			continue
		}

		// Check line range is valid.
		if block.StartLine < 1 || block.EndLine > len(srcLines) {
			result.Invalid = append(result.Invalid, InvalidBlock{
				DocFile:    block.DocFile,
				DocLine:    block.DocLine,
				SourceFile: block.File,
				StartLine:  block.StartLine,
				EndLine:    block.EndLine,
				Reason: fmt.Sprintf("line range %d-%d out of bounds (file has %d lines)",
					block.StartLine, block.EndLine, len(srcLines)),
			})
			continue
		}

		actual := srcLines[block.StartLine-1 : block.EndLine]

		if expanded, abbreviated, err := expanddocs.ExpandAbbreviatedBlock(block, srcLines); abbreviated {
			if err != nil {
				if invalidCoverage[srcPath] == nil {
					invalidCoverage[srcPath] = make(map[int]bool)
				}
				for l := block.StartLine; l <= block.EndLine; l++ {
					invalidCoverage[srcPath][l] = true
				}
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					StartLine:  block.StartLine,
					EndLine:    block.EndLine,
					Reason:     err.Error(),
					Diff:       lineDiff(block.Content, actual),
				})
				continue
			}
			if coverage[srcPath] == nil {
				coverage[srcPath] = make(map[int]bool)
			}
			for l := block.StartLine; l <= block.EndLine; l++ {
				coverage[srcPath][l] = true
			}
			if cfg.Fix {
				fixes = append(fixes, pendingFix{
					docFile: block.DocFile, docLine: block.DocLine,
					oldContent: block.Content, newContent: expanded,
					newStart: block.StartLine, newEnd: block.EndLine,
					removeLines: false,
					reason:      fmt.Sprintf("expanded abbreviated block for lines %d-%d", block.StartLine, block.EndLine),
				})
			}
			continue
		}

		expectedCount := block.EndLine - block.StartLine + 1
		exactMatch := len(block.Content) == expectedCount && linesEqual(block.Content, actual)
		wsMatch := len(block.Content) == expectedCount && linesEqualNormalized(block.Content, actual)

		// If content doesn't match, try with doc-only comment lines stripped.
		if !exactMatch && !wsMatch {
			stripped := stripCommentOnlyLines(block.Content)
			if len(stripped) < len(block.Content) {
				exactMatch = len(stripped) == expectedCount && linesEqual(stripped, actual)
				wsMatch = len(stripped) == expectedCount && linesEqualNormalized(stripped, actual)
			}
		}

		if exactMatch || wsMatch {
			// Content matches (exactly or after normalization).
			if coverage[srcPath] == nil {
				coverage[srcPath] = make(map[int]bool)
			}
			for l := block.StartLine; l <= block.EndLine; l++ {
				coverage[srcPath][l] = true
			}
			// If fixing and content is unique, strip unnecessary lines= annotation.
			if cfg.Fix && len(findAllContent(actual, srcLines, normSrc)) == 1 {
				fixes = append(fixes, pendingFix{
					docFile: block.DocFile, docLine: block.DocLine,
					oldContent: block.Content, newContent: block.Content,
					removeLines: true,
					reason:      fmt.Sprintf("removed lines=%d-%d (content is unique)", block.StartLine, block.EndLine),
				})
			}
			continue
		}

		// Content doesn't match at the stated lines. Try to find it elsewhere
		// in the file (line drift).
		if driftStart := findContent(block.Content, srcLines, normSrc); driftStart >= 0 {
			newStart := driftStart + 1 // convert to 1-based
			newEnd := newStart + len(block.Content) - 1
			if cfg.Fix {
				newContent := srcLines[driftStart : driftStart+len(block.Content)]
				unique := len(findAllContent(newContent, srcLines, normSrc)) == 1
				fixes = append(fixes, pendingFix{
					docFile: block.DocFile, docLine: block.DocLine,
					oldContent: block.Content,
					newContent: newContent,
					newStart:   newStart, newEnd: newEnd,
					removeLines: unique,
					reason:      fmt.Sprintf("line drift: %d-%d -> %d-%d", block.StartLine, block.EndLine, newStart, newEnd),
				})
			} else {
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					StartLine:  block.StartLine,
					EndLine:    block.EndLine,
					Reason: fmt.Sprintf("content found at lines %d-%d instead of %d-%d",
						newStart, newEnd, block.StartLine, block.EndLine),
					Fixable: true,
					FixKind: "drift",
				})
			}
			if coverage[srcPath] == nil {
				coverage[srcPath] = make(map[int]bool)
			}
			for l := newStart; l <= newEnd; l++ {
				coverage[srcPath][l] = true
			}
			continue
		}

		// Try to find a nearby range where ≥70% of lines match (minor edit).
		if bestStart, bestEnd, sim := findBestMatch(block.Content, normSrc, block.StartLine-1); sim >= 0.70 {
			matchedLines := srcLines[bestStart : bestEnd+1]
			newStart := bestStart + 1
			newEnd := bestEnd + 1
			if cfg.Fix {
				unique := len(findAllContent(matchedLines, srcLines, normSrc)) == 1
				fixes = append(fixes, pendingFix{
					docFile: block.DocFile, docLine: block.DocLine,
					oldContent: block.Content, newContent: matchedLines,
					newStart: newStart, newEnd: newEnd,
					removeLines: unique,
					reason:      fmt.Sprintf("minor edit (%.0f%% similar): %d-%d -> %d-%d", sim*100, block.StartLine, block.EndLine, newStart, newEnd),
				})
			} else {
				result.Invalid = append(result.Invalid, InvalidBlock{
					DocFile:    block.DocFile,
					DocLine:    block.DocLine,
					SourceFile: block.File,
					StartLine:  block.StartLine,
					EndLine:    block.EndLine,
					Reason: fmt.Sprintf("minor edit (%.0f%% similar), content now at lines %d-%d",
						sim*100, newStart, newEnd),
					Diff:    lineDiff(block.Content, matchedLines),
					Fixable: true,
					FixKind: "edit",
				})
			}
			if coverage[srcPath] == nil {
				coverage[srcPath] = make(map[int]bool)
			}
			for l := newStart; l <= newEnd; l++ {
				coverage[srcPath][l] = true
			}
			continue
		}

		// Genuine content mismatch — not fixable.
		// Record the invalid block's line range in invalidCoverage so these
		// lines are suppressed from Missing output.
		if invalidCoverage[srcPath] == nil {
			invalidCoverage[srcPath] = make(map[int]bool)
		}
		for l := block.StartLine; l <= block.EndLine; l++ {
			invalidCoverage[srcPath][l] = true
		}
		result.Invalid = append(result.Invalid, InvalidBlock{
			DocFile:    block.DocFile,
			DocLine:    block.DocLine,
			SourceFile: block.File,
			StartLine:  block.StartLine,
			EndLine:    block.EndLine,
			Reason:     "content mismatch",
			Diff:       lineDiff(block.Content, actual),
		})
	}

	// Apply collected fixes bottom-up so line shifts don't affect earlier fixes.
	if cfg.Fix && len(fixes) > 0 {
		sort.Slice(fixes, func(i, j int) bool {
			if fixes[i].docFile != fixes[j].docFile {
				return fixes[i].docFile < fixes[j].docFile
			}
			return fixes[i].docLine > fixes[j].docLine // descending
		})
		for _, f := range fixes {
			rewriteBlock(f.docFile, f.docLine, f.oldContent, f.newContent, f.newStart, f.newEnd, f.removeLines)
			result.Fixed = append(result.Fixed, FixedBlock{
				DocFile: f.docFile,
				DocLine: f.docLine,
				Reason:  f.reason,
			})
		}
	}

	// Warn when prose preceding a code block duplicates a comment inside the block.
	result.Warnings = checkDuplicateComments(allBlocks)

	// Warn when a code block starts with comment lines — these should be prose.
	result.Warnings = append(result.Warnings, checkLeadingComments(allBlocks)...)

	// Check coverage for all referenced source files.
	for absPath := range referencedFiles {
		srcLines := linesCache[absPath]
		if srcLines == nil {
			continue
		}
		commentSet := skippableCache[absPath]

		covered := coverage[absPath]
		invCovered := invalidCoverage[absPath]
		var uncovered []int

		for i := range srcLines {
			lineNum := i + 1
			if commentSet != nil && commentSet[lineNum] {
				continue
			}
			if strings.TrimSpace(srcLines[i]) == "" {
				continue
			}
			if covered != nil && covered[lineNum] {
				continue
			}
			// Suppress lines already reported via an INVALID block.
			if invCovered != nil && invCovered[lineNum] {
				continue
			}
			uncovered = append(uncovered, lineNum)
		}

		displayPath := displayRelPath(absPath, cfg.SourceDirs)
		result.Missing = append(result.Missing, mergeRanges(displayPath, uncovered)...)
	}

	sort.Slice(result.Missing, func(i, j int) bool {
		if result.Missing[i].File != result.Missing[j].File {
			return result.Missing[i].File < result.Missing[j].File
		}
		return result.Missing[i].StartLine < result.Missing[j].StartLine
	})

	return &result, nil
}

func isExcluded(absPath string, sourcePatterns []string, patterns []string) bool {
	rel := displayRelPath(absPath, sourcePatterns)
	for _, pattern := range patterns {
		if filematch.MatchPath(pattern, rel) || filematch.MatchPath(pattern, filepath.Base(rel)) {
			return true
		}
	}
	return false
}

func displayRelPath(absPath string, sourcePatterns []string) string {
	candidates := candidateRelPaths(absPath, sourcePatterns)
	if len(candidates) == 0 {
		return filematch.RelPath(absPath)
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if len(candidate) < len(best) {
			best = candidate
		}
	}
	return best
}

func candidateRelPaths(absPath string, sourcePatterns []string) []string {
	seen := make(map[string]bool)
	var paths []string

	add := func(path string) {
		path = filepath.ToSlash(filepath.Clean(path))
		if path == "." || path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	add(filematch.RelPath(absPath))

	for _, pattern := range sourcePatterns {
		if strings.ContainsAny(pattern, "*?[") {
			continue
		}
		info, err := os.Stat(pattern)
		if err != nil || !info.IsDir() {
			continue
		}
		absRoot, err := filepath.Abs(pattern)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		add(rel)
	}

	return paths
}

// resolveSourceFile looks for file in the matched source files.
func resolveSourceFile(file string, sourceIndex map[string]string) (string, bool) {
	path, ok := sourceIndex[filepath.ToSlash(filepath.Clean(file))]
	return path, ok
}
