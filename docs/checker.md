# Checker — The Check Function

[Back to Overview](overview.md) | Previous: [Types & Configuration](checker-types.md) | Next: [Matching](checker-matching.md)

`Check` is the main entry point. It proceeds in three phases.

## Phase 1: Collect and parse markdown files

First, it walks each docs directory to find `.md` files:

```go file=internal/checker/checker.go
func Check(cfg Config) (*Result, error) {
	// Collect all markdown files from docs dirs.
	var mdFiles []string
	for _, dir := range cfg.DocsDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".md") {
				mdFiles = append(mdFiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking docs dir %s: %w", dir, err)
		}
	}
```

Absolute paths are needed because code blocks use relative `file=` references
that must be joined against each source root. Two separate exclude lists are
built because validation and coverage have different default scopes:

```go file=internal/checker/checker.go
	var absSourceDirs []string
	for _, dir := range cfg.SourceDirs {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
		absSourceDirs = append(absSourceDirs, abs)
	}
```

User-supplied excludes skip both validation and missing coverage. Default
excludes only suppress missing coverage for test code; fixtures and vendored
code are skipped entirely:

```go file=internal/checker/checker.go
	validationExcludes := append(DefaultValidationExclude, cfg.Exclude...)
	coverageExcludes := append(DefaultExclude, cfg.Exclude...)
```

Each markdown file is parsed to extract code blocks:

```go file=internal/checker/checker.go
	var allBlocks []markdown.CodeBlock
	for _, mf := range mdFiles {
		blocks, err := markdown.ParseFile(mf)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", mf, err)
		}
		allBlocks = append(allBlocks, blocks...)
	}
```

## Phase 2: Validate each code block

Two maps track the state: `coverage` records which source lines are covered
by valid blocks, and `referencedFiles` tracks which source files were mentioned.
A third map, `invalidCoverage`, tracks lines referenced by invalid blocks so
they can be suppressed from the missing-coverage output. Three caches avoid
redundant work across blocks referencing the same file:

```go file=internal/checker/checker.go
	coverage := make(map[string]map[int]bool)
	// invalidCoverage tracks lines referenced by INVALID blocks (with lines=)
	// so they can be suppressed from Missing output.
	invalidCoverage := make(map[string]map[int]bool)
	// referencedFiles tracks absolute paths of source files referenced by any code block.
	referencedFiles := make(map[string]bool)

	// Caches to avoid redundant work across blocks referencing the same file.
	resolveCache := make(map[string]string)         // block.File -> absPath (empty if not found)
	linesCache := make(map[string][]string)          // absPath -> raw lines
	normCache := make(map[string][]string)           // absPath -> normalized lines
	skippableCache := make(map[string]map[int]bool)  // absPath -> skippable line set
```

Pending fixes are collected during the loop and applied afterward, since
modifying a markdown file mid-loop would shift line numbers for subsequent
blocks:

```go file=internal/checker/checker.go
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
```

For each code block, we resolve the `file=` path against the source
directories (using a cache). Files in the validation exclude list are skipped
entirely:

```go file=internal/checker/checker.go
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

		if isExcluded(srcPath, validationExcludes) {
			continue
		}
```

Then we decide whether the referenced file should participate in missing
coverage. Test code can still be validated, but it is not added to
`referencedFiles` unless it is outside the coverage exclude list:

```go file=internal/checker/checker.go
		if !isExcluded(srcPath, absSourceDirs, coverageExcludes) {
			referencedFiles[srcPath] = true
		}
```

Source lines are read lazily and cached across blocks referencing the same file:

```go file=internal/checker/checker.go
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
```

Before the normal verbatim matching path, Mermaid diagrams get special
handling. A `mermaid` fence can describe a function or method structurally
instead of embedding its source verbatim. In that case the checker resolves the
named declaration and marks that declaration range as covered:

```go file=internal/checker/checker.go
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
```

### Blocks without line numbers

When a block has no `lines=` annotation, the checker first tries abbreviated
expansion, then exact content matching (with comment stripping as a fallback),
then fuzzy matching, and finally reports ambiguous matches:

```go file=internal/checker/checker.go
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
							reason: fmt.Sprintf("minor edit (%.0f%% similar): content now at lines %d-%d", sim*100, newStart, newEnd),
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
```

### Blocks with line numbers

When `lines=` is present, the checker validates the line range is in bounds,
extracts the actual source lines, and tries several matching strategies:

```go file=internal/checker/checker.go
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
```

Abbreviated blocks with `lines=` get expanded and validated against the stated
range. If expansion fails, the block's lines are recorded in `invalidCoverage`
so they don't appear as missing:

```go file=internal/checker/checker.go
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
```

Content is compared both exactly and with whitespace normalization. Doc-only
comment lines are stripped as a second pass if neither matches:

```go file=internal/checker/checker.go
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
```

When content matches, the lines are recorded as covered. If fixing and the
content is unique in the file, the `lines=` annotation is stripped:

```go file=internal/checker/checker.go
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
					oldContent:  block.Content, newContent: block.Content,
					removeLines: true,
					reason:      fmt.Sprintf("removed lines=%d-%d (content is unique)", block.StartLine, block.EndLine),
				})
			}
			continue
		}
```

When content doesn't match at the stated lines, drift detection searches the
entire file. If that fails, fuzzy matching looks for a similar block nearby:

```go file=internal/checker/checker.go
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
					newStart: newStart, newEnd: newEnd,
					removeLines: unique,
					reason: fmt.Sprintf("line drift: %d-%d -> %d-%d", block.StartLine, block.EndLine, newStart, newEnd),
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
					reason: fmt.Sprintf("minor edit (%.0f%% similar): %d-%d -> %d-%d", sim*100, block.StartLine, block.EndLine, newStart, newEnd),
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
```

When nothing matches, the block is a genuine content mismatch. Its lines are
recorded in `invalidCoverage` so they don't show up as missing:

```go file=internal/checker/checker.go
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
```

### Fix application

After all blocks are validated, pending fixes are applied bottom-up (by
descending doc line) so that earlier line numbers aren't shifted by later
rewrites:

```go file=internal/checker/checker.go
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
```

## Phase 3: Coverage analysis

After all blocks have been validated, we check every referenced source file
for uncovered lines. Comments, blank lines, and lines covered by invalid blocks
are excluded:

```go file=internal/checker/checker.go
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

		displayPath := displayRelPath(absPath)
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
```

## File exclusion

The `isExcluded` function checks a file's relative path against all exclusion
patterns. It handles both simple globs and `**` patterns:

```go file=internal/checker/checker.go
func isExcluded(absPath string, sourceDirs []string, patterns []string) bool {
	for _, sd := range sourceDirs {
		rel, err := filepath.Rel(sd, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, rel); matched {
				return true
			}
			if matched, _ := filepath.Match(pattern, filepath.Base(rel)); matched {
				return true
			}
			// Handle ** patterns by checking if any path segment matches.
			if strings.Contains(pattern, "**") {
				if matchDoublestar(pattern, rel) {
					return true
				}
			}
		}
	}
	return false
}
```

The `matchDoublestar` helper provides basic `**` glob support by splitting the
pattern on `**` and checking if any path segment matches the prefix, with the
remaining path matching the suffix:

```go file=internal/checker/checker.go
func matchDoublestar(pattern, path string) bool {
	// Split on ** and check prefix/suffix matching.
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return false
	}
	prefix := strings.TrimSuffix(parts[0], string(filepath.Separator))
	suffix := strings.TrimPrefix(parts[1], string(filepath.Separator))

	// Check if path contains the prefix directory.
	pathParts := strings.Split(path, string(filepath.Separator))
	for i, part := range pathParts {
		if prefix == "" || part == prefix {
			// Check remaining path against suffix.
			remaining := strings.Join(pathParts[i+1:], string(filepath.Separator))
			if suffix == "" {
				return true
			}
			if matched, _ := filepath.Match(suffix, remaining); matched {
				return true
			}
			if matched, _ := filepath.Match(suffix, filepath.Base(remaining)); matched {
				return true
			}
		}
	}
	return false
}
```

## Display paths

`displayRelPath` converts an absolute path back to a relative path for
user-friendly output:

```go file=internal/checker/checker.go
func displayRelPath(absPath string, sourceDirs []string) string {
	for _, sd := range sourceDirs {
		if rel, err := filepath.Rel(sd, absPath); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return absPath
}
```

## Source file resolution

`resolveSourceFile` searches the source directories in order and returns the
first path where the file exists:

```go file=internal/checker/checker.go
func resolveSourceFile(file string, sourceDirs []string) (string, bool) {
	for _, dir := range sourceDirs {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
```

Continue to [Matching](checker-matching.md) to see how content matching and drift
detection work.
