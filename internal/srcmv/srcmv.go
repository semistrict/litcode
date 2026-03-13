package srcmv

import (
	"fmt"
	"os"
	"strings"

	"github.com/semistrict/litcode/internal/comments"
	sitter "github.com/smacker/go-tree-sitter"
)

// Move extracts the top-level declaration at srcFile:srcLine:srcCol (1-based)
// along with its attached doc comments, removes it from the source file, and
// inserts it into destFile at destLine (1-based). If destFile does not exist it
// is created. srcFile and destFile may be the same file.
func Move(srcFile string, srcLine, srcCol int, destFile string, destLine int) error {
	srcBytes, err := os.ReadFile(srcFile)
	if err != nil {
		return fmt.Errorf("reading source: %w", err)
	}

	tree := comments.ParseTree(srcFile, srcBytes)
	if tree == nil {
		return fmt.Errorf("unsupported or unparseable file: %s", srcFile)
	}
	defer tree.Close()

	root := tree.RootNode()

	// Find the top-level named node at srcLine:srcCol (convert to 0-based).
	node := findTopLevelNode(root, uint32(srcLine-1), uint32(srcCol-1))
	if node == nil {
		return fmt.Errorf("no top-level declaration found at %s:%d:%d", srcFile, srcLine, srcCol)
	}

	// Collect attached doc comments (contiguous comment siblings above the node).
	commentStart := collectDocComments(root, node)

	// Determine line range to extract (0-based).
	extractStart := int(commentStart.StartPoint().Row)
	extractEnd := int(node.EndPoint().Row)
	if node.EndPoint().Column == 0 && extractEnd > int(node.StartPoint().Row) {
		extractEnd--
	}

	srcLines := strings.Split(string(srcBytes), "\n")

	// Extract the block.
	extracted := make([]string, extractEnd-extractStart+1)
	copy(extracted, srcLines[extractStart:extractEnd+1])

	// Remove from source.
	remaining := make([]string, 0, len(srcLines)-(extractEnd-extractStart+1))
	remaining = append(remaining, srcLines[:extractStart]...)
	remaining = append(remaining, srcLines[extractEnd+1:]...)

	// Clean up double blank lines left by removal.
	remaining = collapseBlankLines(remaining)

	sameFile := samePath(srcFile, destFile)

	// Adjust destLine for same-file moves.
	if sameFile {
		removedCount := extractEnd - extractStart + 1
		destIdx := destLine - 1 // 0-based
		if destIdx >= extractStart && destIdx <= extractEnd {
			return fmt.Errorf("destination line %d falls within the extracted range (%d-%d)",
				destLine, extractStart+1, extractEnd+1)
		}
		if destIdx > extractStart {
			destLine -= removedCount
		}
	}

	// Read or reuse destination lines.
	var destLines []string
	if sameFile {
		destLines = remaining
	} else {
		destBytes, err := os.ReadFile(destFile)
		if err != nil {
			if os.IsNotExist(err) {
				destLines = []string{}
			} else {
				return fmt.Errorf("reading destination: %w", err)
			}
		} else {
			destLines = strings.Split(string(destBytes), "\n")
		}
	}

	// Insert at destLine (1-based). Clamp to valid range.
	insertIdx := destLine - 1
	if insertIdx < 0 {
		return fmt.Errorf("destination line must be >= 1, got %d", destLine)
	}
	if insertIdx > len(destLines) {
		insertIdx = len(destLines)
	}

	// Build the block to insert with blank-line separators.
	var block []string

	// Add leading blank line if preceding line is non-blank.
	if insertIdx > 0 && insertIdx <= len(destLines) && strings.TrimSpace(destLines[insertIdx-1]) != "" {
		block = append(block, "")
	}

	block = append(block, extracted...)

	// Add trailing blank line if following line is non-blank.
	if insertIdx < len(destLines) && strings.TrimSpace(destLines[insertIdx]) != "" {
		block = append(block, "")
	}

	// Splice into destLines.
	result := make([]string, 0, len(destLines)+len(block))
	result = append(result, destLines[:insertIdx]...)
	result = append(result, block...)
	result = append(result, destLines[insertIdx:]...)

	result = collapseBlankLines(result)

	// Write files.
	if sameFile {
		return writeFile(srcFile, result)
	}

	if err := writeFile(srcFile, remaining); err != nil {
		return err
	}
	return writeFile(destFile, result)
}

// findTopLevelNode finds a top-level named child of root whose line range
// contains the given 0-based row and whose start column matches col (or col is
// within the node).
func findTopLevelNode(root *sitter.Node, row, col uint32) *sitter.Node {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		startRow := child.StartPoint().Row
		endRow := child.EndPoint().Row
		if child.EndPoint().Column == 0 && endRow > startRow {
			endRow--
		}
		if row >= startRow && row <= endRow {
			// If a column was specified (non-zero), check it falls within the node on the start row.
			if row == startRow && col > 0 && col < child.StartPoint().Column {
				continue
			}
			// Skip non-movable nodes: comments, package, imports.
			tp := child.Type()
			if strings.Contains(tp, "comment") ||
				strings.Contains(tp, "package") ||
				strings.Contains(tp, "import") {
				continue
			}
			return child
		}
	}
	return nil
}

// collectDocComments walks backwards through root's named children from the
// node's position and returns the earliest contiguous comment node that is
// attached (no blank-line gap). If no comments are attached, returns node itself.
func collectDocComments(root *sitter.Node, node *sitter.Node) *sitter.Node {
	// Find the index of node among root's named children.
	idx := -1
	for i := 0; i < int(root.NamedChildCount()); i++ {
		if root.NamedChild(i) == node {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return node
	}

	earliest := node
	prevEndRow := int(node.StartPoint().Row) // 0-based row where the node starts

	for i := idx - 1; i >= 0; i-- {
		sibling := root.NamedChild(i)
		tp := sibling.Type()
		if !strings.Contains(tp, "comment") {
			break
		}
		sibEndRow := int(sibling.EndPoint().Row)
		if sibling.EndPoint().Column == 0 && sibEndRow > int(sibling.StartPoint().Row) {
			sibEndRow--
		}
		// Check adjacency: no blank line between this comment's end and the
		// previous block's start.
		if prevEndRow-sibEndRow > 1 {
			break
		}
		earliest = sibling
		prevEndRow = int(sibling.StartPoint().Row)
	}
	return earliest
}

// collapseBlankLines removes consecutive blank lines, keeping at most one.
func collapseBlankLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && prevBlank {
			continue
		}
		result = append(result, line)
		prevBlank = blank
	}
	// Trim trailing blank lines, keep exactly one empty line at end
	// (which becomes the trailing newline when joined).
	for len(result) > 1 && strings.TrimSpace(result[len(result)-1]) == "" &&
		strings.TrimSpace(result[len(result)-2]) == "" {
		result = result[:len(result)-1]
	}
	return result
}

func samePath(a, b string) bool {
	// Attempt to resolve to absolute paths for comparison.
	absA, errA := resolveAbs(a)
	absB, errB := resolveAbs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return absA == absB
}

func resolveAbs(path string) (string, error) {
	abs, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		path = abs + "/" + path
	}
	// Clean the path.
	cleaned := strings.Builder{}
	parts := strings.Split(path, "/")
	stack := []string{}
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		stack = append(stack, p)
	}
	cleaned.WriteString("/")
	cleaned.WriteString(strings.Join(stack, "/"))
	return cleaned.String(), nil
}

func writeFile(path string, lines []string) error {
	content := strings.Join(lines, "\n")
	// Ensure trailing newline.
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
