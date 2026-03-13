package markdown

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// CodeBlock represents a fenced code block that references source file lines.
type CodeBlock struct {
	Language  string
	File      string   // referenced source file path (relative to root)
	StartLine int      // start line in source file (1-based, inclusive; 0 if omitted)
	EndLine   int      // end line in source file (1-based, inclusive; 0 if omitted)
	Content   []string // lines of content in the code block
	DocFile   string   // which .md file this block is in
	DocLine   int      // line number in the .md file where the fence opens
}

// infoRegex matches: lang file=path lines=N or lines=N-M (lines= is optional)
var infoRegex = regexp.MustCompile(`^(\w+)\s+file=(\S+)(?:\s+lines=(\d+)(?:-(\d+))?)?$`)

// ParseFile extracts code blocks with file/line references from a markdown file.
func ParseFile(path string) (_ []CodeBlock, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var blocks []CodeBlock
	scanner := bufio.NewScanner(f)
	lineNum := 0

	var current *CodeBlock

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if current == nil {
			// Per CommonMark, fenced code blocks may be indented 0-3 spaces.
			// Lines indented 4+ are indented code blocks and should be ignored.
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if indent <= 3 && strings.HasPrefix(trimmed, "```") {
				info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				if m := infoRegex.FindStringSubmatch(info); m != nil {
					var start, end int
					if m[3] != "" {
						start, _ = strconv.Atoi(m[3])
						end = start
						if m[4] != "" {
							end, _ = strconv.Atoi(m[4])
						}
					}
					current = &CodeBlock{
						Language:  m[1],
						File:      m[2],
						StartLine: start,
						EndLine:   end,
						DocFile:   path,
						DocLine:   lineNum,
					}
				}
			}
		} else {
			if trimmed == "```" {
				blocks = append(blocks, *current)
				current = nil
			} else {
				current.Content = append(current.Content, line)
			}
		}
	}

	return blocks, scanner.Err()
}
