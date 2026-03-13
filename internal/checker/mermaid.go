package checker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/semistrict/litcode/internal/markdown"
)

var mermaidSymbolRegex = regexp.MustCompile(`(?i)\b(?:function|method)\s*:\s*([A-Za-z_][\w.:()*-]*)`)

var mermaidLangForExt = map[string]*sitter.Language{
	".go":   golang.GetLanguage(),
	".js":   javascript.GetLanguage(),
	".ts":   typescript.GetLanguage(),
	".py":   python.GetLanguage(),
	".rs":   rust.GetLanguage(),
	".c":    c.GetLanguage(),
	".h":    c.GetLanguage(),
	".cpp":  cpp.GetLanguage(),
	".cc":   cpp.GetLanguage(),
	".cxx":  cpp.GetLanguage(),
	".hpp":  cpp.GetLanguage(),
	".java": java.GetLanguage(),
	".rb":   ruby.GetLanguage(),
}

var mermaidDeclarationNodeTypes = map[string]bool{
	"constructor_declaration": true,
	"function_declaration":    true,
	"function_definition":     true,
	"function_item":           true,
	"method":                  true,
	"method_declaration":      true,
	"method_definition":       true,
	"singleton_method":        true,
}

func mermaidCoveredLines(block markdown.CodeBlock, srcPath string, totalLines int) ([]int, error) {
	symbol, ok := mermaidSymbol(block.Content)
	if !ok {
		if block.StartLine != 0 {
			return expandCoveredRange(block.StartLine, block.EndLine, totalLines)
		}
		return nil, fmt.Errorf("mermaid block must name the function or method it explains, e.g. `title function: classify`")
	}

	start, end, err := findDeclarationRange(srcPath, symbol)
	if err != nil {
		return nil, err
	}
	return expandCoveredRange(start, end, totalLines)
}

func mermaidSymbol(lines []string) (string, bool) {
	for _, line := range lines {
		match := mermaidSymbolRegex.FindStringSubmatch(line)
		if len(match) == 2 {
			return normalizeSymbolName(match[1]), true
		}
	}
	return "", false
}

func normalizeSymbolName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "\"'`")
	name = strings.TrimSuffix(name, "()")
	name = strings.ReplaceAll(name, "::", ".")
	parts := strings.Split(name, ".")
	name = parts[len(parts)-1]
	return strings.Trim(name, "*() ")
}

func expandCoveredRange(start, end, totalLines int) ([]int, error) {
	if start < 1 || end < start || end > totalLines {
		return nil, fmt.Errorf("declaration lines %d-%d out of bounds (file has %d lines)", start, end, totalLines)
	}

	covered := make([]int, 0, end-start+1)
	for lineNum := start; lineNum <= end; lineNum++ {
		covered = append(covered, lineNum)
	}
	return covered, nil
}

func findDeclarationRange(path, symbol string) (int, int, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read source file: %v", err)
	}

	lang, ok := mermaidLangForExt[filepath.Ext(path)]
	if !ok {
		return 0, 0, fmt.Errorf("mermaid symbol coverage is unsupported for %s files", filepath.Ext(path))
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse source file: %v", err)
	}
	defer tree.Close()

	target := normalizeSymbolName(symbol)
	var matches [][2]int

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if mermaidDeclarationNodeTypes[node.Type()] {
			if name := symbolNameForNode(node, source); normalizeSymbolName(name) == target {
				start := int(node.StartPoint().Row) + 1
				end := int(node.EndPoint().Row) + 1
				if node.EndPoint().Column == 0 && end > start {
					end--
				}
				matches = append(matches, [2]int{start, end})
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(tree.RootNode())

	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("could not find a declaration named %q in %s", target, filepath.Base(path))
	}
	if len(matches) > 1 {
		return 0, 0, fmt.Errorf("found multiple declarations named %q in %s", target, filepath.Base(path))
	}
	return matches[0][0], matches[0][1], nil
}

func symbolNameForNode(node *sitter.Node, source []byte) string {
	if name := node.ChildByFieldName("name"); name != nil {
		return strings.TrimSpace(name.Content(source))
	}

	if declarator := node.ChildByFieldName("declarator"); declarator != nil {
		if name := firstIdentifierName(declarator, source); name != "" {
			return name
		}
	}

	return firstIdentifierName(node, source)
}

func firstIdentifierName(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	if node.ChildCount() == 0 {
		switch node.Type() {
		case "constant", "destructor_name", "field_identifier", "identifier", "property_identifier", "type_identifier":
			return strings.TrimSpace(node.Content(source))
		}
		return ""
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if name := firstIdentifierName(node.Child(i), source); name != "" {
			return name
		}
	}
	return ""
}
