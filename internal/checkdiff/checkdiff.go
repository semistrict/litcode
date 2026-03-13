package checkdiff

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/semistrict/litcode/internal/checker"
	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/markdown"
)

type Config struct {
	DocsDirs   []string
	SourceDirs []string
	Lenient    []string
	Exclude    []string
	GitArgs    []string
}

type Result struct {
	Invalid        []checker.InvalidBlock
	Warnings       []checker.Warning
	MissingAdded   []checker.MissingRange
	MissingRemoved []RemovedMention
}

type RemovedMention struct {
	File   string
	Symbol string
}

type fileDiff struct {
	OldPath string
	NewPath string
	Added   []lineRange
	Removed []removedChunk
}

type lineRange struct {
	Start int
	End   int
}

type removedChunk struct {
	File   string
	Symbol string
}

var (
	hunkHeaderRE   = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?:\s?(.*))?$`)
	shaLikeRE      = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	removalVerbRE  = regexp.MustCompile(`\b(remove|removed|delete|deleted|drop|dropped)\b`)
	symbolPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\bfunc\s*(?:\([^)]*\)\s*)?([A-Za-z_][\w]*)\s*\(`),
		regexp.MustCompile(`\bfunction\s+([A-Za-z_][\w]*)\s*\(`),
		regexp.MustCompile(`\bdef\s+([A-Za-z_][\w]*)\s*\(`),
		regexp.MustCompile(`\bclass\s+([A-Za-z_][\w]*)\b`),
		regexp.MustCompile(`\b(?:type|struct|interface|enum|trait)\s+([A-Za-z_][\w]*)\b`),
		regexp.MustCompile(`\b(?:const|let|var)\s+([A-Za-z_][\w]*)\b`),
		regexp.MustCompile(`\b([A-Za-z_][\w]*)\s*:=\s*func\b`),
		regexp.MustCompile(`\b([A-Za-z_][\w]*)\s*[:=]\s*(?:async\s+)?function\b`),
		regexp.MustCompile(`\b([A-Za-z_][\w]*)\s*[:=]\s*(?:async\s+)?\([^)]*\)\s*=>`),
	}
)

func Check(cfg Config) (*Result, error) {
	diffText, err := gitDiffOutput(cfg.GitArgs)
	if err != nil {
		return nil, err
	}

	diffs, err := parseDiff(diffText)
	if err != nil {
		return nil, err
	}

	docFiles, err := collectDocFiles(cfg.DocsDirs)
	if err != nil {
		return nil, err
	}

	sourceIndex, err := filematch.Index(cfg.SourceDirs)
	if err != nil {
		return nil, fmt.Errorf("collecting source files: %w", err)
	}

	referenced, err := collectReferencedFiles(docFiles, sourceIndex)
	if err != nil {
		return nil, err
	}

	paragraphs, err := collectProseParagraphs(docFiles)
	if err != nil {
		return nil, err
	}

	changedFiles := changedFilesForValidation(diffs, cfg.SourceDirs, cfg.Lenient, cfg.Exclude, sourceIndex)

	checkResult := &checker.Result{}
	if len(changedFiles) > 0 {
		checkResult, err = checker.Check(checker.Config{
			DocsDirs:   cfg.DocsDirs,
			SourceDirs: cfg.SourceDirs,
			Lenient:    cfg.Lenient,
			Exclude:    cfg.Exclude,
			Files:      changedFiles,
		})
		if err != nil {
			return nil, err
		}
	}

	return &Result{
		Invalid:        checkResult.Invalid,
		Warnings:       checkResult.Warnings,
		MissingAdded:   missingAddedRanges(diffs, checkResult.Missing, referenced, cfg.SourceDirs, cfg.Lenient, cfg.Exclude, sourceIndex),
		MissingRemoved: missingRemovedMentions(diffs, paragraphs, cfg.SourceDirs, cfg.Lenient, cfg.Exclude),
	}, nil
}

func gitDiffOutput(args []string) (string, error) {
	cmdArgs := []string{"diff", "--unified=0", "--no-ext-diff"}
	if len(args) == 1 && shaLikeRE.MatchString(args[0]) {
		cmdArgs = []string{"show", "--format=", "--unified=0", "--no-ext-diff", args[0]}
	} else if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("running %q: %w\n%s", "git "+strings.Join(cmdArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseDiff(diffText string) ([]fileDiff, error) {
	if strings.TrimSpace(diffText) == "" {
		return nil, nil
	}

	lines := strings.Split(diffText, "\n")
	var diffs []fileDiff
	var current *fileDiff

	flush := func() {
		if current == nil {
			return
		}
		diffs = append(diffs, *current)
		current = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			current = &fileDiff{}
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				current.OldPath = normalizeDiffPath(fields[2])
				current.NewPath = normalizeDiffPath(fields[3])
			}
		case current == nil:
			continue
		case strings.HasPrefix(line, "--- "):
			if path := normalizeDiffPath(strings.TrimPrefix(line, "--- ")); path != "" {
				current.OldPath = path
			}
		case strings.HasPrefix(line, "+++ "):
			if path := normalizeDiffPath(strings.TrimPrefix(line, "+++ ")); path != "" {
				current.NewPath = path
			}
		case strings.HasPrefix(line, "@@ "):
			_, oldCount, newStart, newCount, header, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			if newCount > 0 {
				current.Added = append(current.Added, lineRange{
					Start: newStart,
					End:   newStart + newCount - 1,
				})
			}

			var removedLines []string
			for i+1 < len(lines) {
				next := lines[i+1]
				if strings.HasPrefix(next, "diff --git ") || strings.HasPrefix(next, "@@ ") {
					break
				}
				i++
				if strings.HasPrefix(next, "--- ") || strings.HasPrefix(next, "+++ ") || strings.HasPrefix(next, `\ No newline at end of file`) {
					continue
				}
				if strings.HasPrefix(next, "-") {
					removedLines = append(removedLines, next[1:])
				}
			}

			if oldCount > 0 && len(removedLines) > 0 {
				current.Removed = append(current.Removed, removedChunk{
					File:   removedFilePath(*current),
					Symbol: removedSymbol(header, removedLines),
				})
			}
		}
	}

	flush()
	return diffs, nil
}

func parseHunkHeader(line string) (oldStart, oldCount, newStart, newCount int, header string, err error) {
	m := hunkHeaderRE.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, 0, 0, "", fmt.Errorf("parsing hunk header %q", line)
	}
	oldStart = atoiDefault(m[1], 0)
	oldCount = atoiDefault(m[2], 1)
	newStart = atoiDefault(m[3], 0)
	newCount = atoiDefault(m[4], 1)
	header = strings.TrimSpace(m[5])
	return oldStart, oldCount, newStart, newCount, header, nil
}

func normalizeDiffPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/dev/null" {
		return ""
	}
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return filepath.ToSlash(filepath.Clean(path))
}

func removedFilePath(diff fileDiff) string {
	if diff.NewPath != "" {
		return diff.NewPath
	}
	return diff.OldPath
}

func removedSymbol(header string, removedLines []string) string {
	texts := append([]string{}, removedLines...)
	if header != "" {
		texts = append(texts, header)
	}
	for _, text := range texts {
		for _, re := range symbolPatterns {
			if m := re.FindStringSubmatch(text); len(m) == 2 {
				return m[1]
			}
		}
	}
	return ""
}

func collectDocFiles(patterns []string) ([]string, error) {
	matches, err := filematch.Collect(patterns, func(relPath string) bool {
		return strings.HasSuffix(relPath, ".md")
	})
	if err != nil {
		return nil, fmt.Errorf("collecting docs: %w", err)
	}

	docFiles := make([]string, 0, len(matches))
	for _, match := range matches {
		docFiles = append(docFiles, match.AbsPath)
	}
	return docFiles, nil
}

func collectReferencedFiles(docFiles []string, sourceIndex map[string]string) (map[string]bool, error) {
	referenced := make(map[string]bool)
	for _, docFile := range docFiles {
		blocks, err := markdown.ParseFile(docFile)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", docFile, err)
		}
		for _, block := range blocks {
			addCanonicalKeys(referenced, filepath.ToSlash(filepath.Clean(block.File)), sourceIndex)
		}
	}
	return referenced, nil
}

func collectProseParagraphs(docFiles []string) ([]string, error) {
	var paragraphs []string
	for _, docFile := range docFiles {
		f, err := os.Open(docFile)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", docFile, err)
		}

		scanner := bufio.NewScanner(f)
		var current []string
		inFence := false
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if indent <= 3 && strings.HasPrefix(trimmed, "```") {
				if len(current) > 0 {
					paragraphs = append(paragraphs, normalizeParagraph(strings.Join(current, " ")))
					current = nil
				}
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if trimmed == "" {
				if len(current) > 0 {
					paragraphs = append(paragraphs, normalizeParagraph(strings.Join(current, " ")))
					current = nil
				}
				continue
			}
			current = append(current, trimmed)
		}
		if err := scanner.Err(); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("scanning %s: %w", docFile, err)
		}
		if len(current) > 0 {
			paragraphs = append(paragraphs, normalizeParagraph(strings.Join(current, " ")))
		}
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("closing %s: %w", docFile, err)
		}
	}
	return paragraphs, nil
}

func normalizeParagraph(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func changedFilesForValidation(diffs []fileDiff, sourcePatterns, lenient, exclude []string, sourceIndex map[string]string) []string {
	files := make(map[string]bool)
	for _, diff := range diffs {
		if diff.NewPath == "" {
			continue
		}
		if !matchesSource(diff.NewPath, sourcePatterns) || isCoverageExcluded(diff.NewPath, lenient, exclude) {
			continue
		}
		abs, ok := sourceIndex[diff.NewPath]
		if !ok {
			continue
		}
		files[abs] = true
	}

	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func missingAddedRanges(diffs []fileDiff, checkerMissing []checker.MissingRange, referenced map[string]bool, sourcePatterns, lenient, exclude []string, sourceIndex map[string]string) []checker.MissingRange {
	missingByFile := make(map[string]map[int]bool)
	for _, mr := range checkerMissing {
		keys := canonicalKeys(mr.File, sourceIndex)
		if len(keys) == 0 {
			keys = []string{filepath.ToSlash(filepath.Clean(mr.File))}
		}
		for _, key := range keys {
			if missingByFile[key] == nil {
				missingByFile[key] = make(map[int]bool)
			}
			for line := mr.StartLine; line <= mr.EndLine; line++ {
				missingByFile[key][line] = true
			}
		}
	}

	var result []checker.MissingRange
	for _, diff := range diffs {
		if diff.NewPath == "" || len(diff.Added) == 0 {
			continue
		}
		if !matchesSource(diff.NewPath, sourcePatterns) || isCoverageExcluded(diff.NewPath, lenient, exclude) {
			continue
		}

		keys := canonicalKeys(diff.NewPath, sourceIndex)
		hasReference := false
		for _, key := range keys {
			if referenced[key] {
				hasReference = true
				break
			}
		}

		if !hasReference {
			for _, r := range diff.Added {
				result = append(result, checker.MissingRange{
					File:      diff.NewPath,
					StartLine: r.Start,
					EndLine:   r.End,
				})
			}
			continue
		}

		missingLines := make(map[int]bool)
		for _, key := range keys {
			for line := range missingByFile[key] {
				missingLines[line] = true
			}
		}

		var uncovered []int
		for _, r := range diff.Added {
			for line := r.Start; line <= r.End; line++ {
				if missingLines[line] {
					uncovered = append(uncovered, line)
				}
			}
		}
		result = append(result, mergeLines(diff.NewPath, uncovered)...)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		return result[i].StartLine < result[j].StartLine
	})
	return result
}

func missingRemovedMentions(diffs []fileDiff, paragraphs []string, sourcePatterns, lenient, exclude []string) []RemovedMention {
	seen := make(map[string]bool)
	var missing []RemovedMention

	for _, diff := range diffs {
		for _, removed := range diff.Removed {
			if removed.File == "" || !matchesSource(removed.File, sourcePatterns) || isCoverageExcluded(removed.File, lenient, exclude) {
				continue
			}
			key := removed.File + "\x00" + removed.Symbol
			if seen[key] {
				continue
			}
			seen[key] = true

			if hasRemovalMention(paragraphs, removed.File, removed.Symbol) {
				continue
			}
			missing = append(missing, RemovedMention(removed))
		}
	}

	sort.Slice(missing, func(i, j int) bool {
		if missing[i].File != missing[j].File {
			return missing[i].File < missing[j].File
		}
		return missing[i].Symbol < missing[j].Symbol
	})
	return missing
}

func hasRemovalMention(paragraphs []string, file, symbol string) bool {
	file = strings.ToLower(file)
	base := strings.ToLower(filepath.Base(file))
	symbol = strings.ToLower(symbol)
	for _, paragraph := range paragraphs {
		if !removalVerbRE.MatchString(paragraph) {
			continue
		}
		if !strings.Contains(paragraph, file) && !strings.Contains(paragraph, base) {
			continue
		}
		if symbol != "" && !strings.Contains(paragraph, symbol) {
			continue
		}
		return true
	}
	return false
}

func canonicalKeys(path string, sourceIndex map[string]string) []string {
	seen := make(map[string]bool)
	var keys []string
	add := func(key string) {
		key = filepath.ToSlash(filepath.Clean(key))
		if key == "." || key == "" || seen[key] {
			return
		}
		seen[key] = true
		keys = append(keys, key)
	}

	add(path)
	if abs, ok := sourceIndex[path]; ok {
		add(abs)
	}
	return keys
}

func addCanonicalKeys(dst map[string]bool, path string, sourceIndex map[string]string) {
	for _, key := range canonicalKeys(path, sourceIndex) {
		dst[key] = true
	}
}

func mergeLines(file string, lines []int) []checker.MissingRange {
	if len(lines) == 0 {
		return nil
	}
	sort.Ints(lines)
	var merged []checker.MissingRange
	start := lines[0]
	end := lines[0]
	for _, line := range lines[1:] {
		if line == end || line == end+1 {
			if line > end {
				end = line
			}
			continue
		}
		merged = append(merged, checker.MissingRange{File: file, StartLine: start, EndLine: end})
		start = line
		end = line
	}
	merged = append(merged, checker.MissingRange{File: file, StartLine: start, EndLine: end})
	return merged
}

func matchesSource(path string, sourcePatterns []string) bool {
	base := filepath.Base(path)
	for _, pattern := range sourcePatterns {
		pattern = filepath.ToSlash(filepath.Clean(pattern))
		if strings.ContainsAny(pattern, "*?[") {
			if filematch.MatchPath(pattern, path) || filematch.MatchPath(pattern, base) {
				return true
			}
			continue
		}
		if strings.HasPrefix(path, pattern+"/") || path == pattern || base == pattern {
			return true
		}
	}
	return false
}

func isCoverageExcluded(path string, lenient, exclude []string) bool {
	base := filepath.Base(path)
	patterns := append([]string{}, checker.DefaultExclude...)
	patterns = append(patterns, lenient...)
	patterns = append(patterns, exclude...)
	for _, pattern := range patterns {
		if filematch.MatchPath(pattern, path) || filematch.MatchPath(pattern, base) {
			return true
		}
	}
	return false
}

func atoiDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
