package comments

import (
	"context"
	"path/filepath"
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
)

var extToLang = map[string]*sitter.Language{
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

// LanguageForFilename returns the tree-sitter language for a file based on its
// extension, or nil if the extension is not supported.
func LanguageForFilename(filename string) *sitter.Language {
	return extToLang[filepath.Ext(filename)]
}

// ParseTree parses source code using the tree-sitter grammar appropriate for
// filename's extension. Returns nil if the extension is unsupported or parsing
// fails.
func ParseTree(filename string, source []byte) *sitter.Tree {
	lang := LanguageForFilename(filename)
	if lang == nil {
		return nil
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil
	}
	return tree
}

// skippableNodeTypes are AST node types whose lines don't require doc coverage.
var skippableNodeTypes = map[string]bool{
	"comment":                  true,
	"line_comment":             true,
	"block_comment":            true,
	"package_clause":           true,
	"package_declaration":      true,
	"import_declaration":       true,
	"import_spec":              true,
	"import_spec_list":         true,
	"import_statement":         true,
	"import_from_statement":    true,
	"use_declaration":          true,
	"extern_crate_declaration": true,
	"preproc_include":          true,
	"require":                  true,
	"require_relative":         true,
}

// SkippableLines returns the set of 1-based line numbers that don't require
// documentation coverage: blank lines, comments, package declarations, and
// import statements.
// Returns nil for unsupported file extensions (all lines need coverage).
func SkippableLines(filename string, source []byte) map[int]bool {
	tree := ParseTree(filename, source)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	lines := strings.Split(string(source), "\n")
	root := tree.RootNode()
	skipSet := collectSkippableLines(root)
	syntaxOnly := collectSyntaxOnlyLines(root, len(lines))

	result := make(map[int]bool)
	for i, line := range lines {
		lineNum := i + 1
		if strings.TrimSpace(line) == "" || skipSet[lineNum] || syntaxOnly[lineNum] {
			result[lineNum] = true
		}
	}
	return result
}

// TopLevelCommentLines returns 1-based line numbers occupied by comment nodes
// whose ancestors are only the root node and optional ERROR wrappers. These
// comments sit at the top level of the parsed snippet rather than nested inside
// declarations or statements.
func TopLevelCommentLines(filename string, source []byte) map[int]bool {
	tree := ParseTree(filename, source)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	root := tree.RootNode()
	lines := make(map[int]bool)
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		tp := n.Type()
		if strings.Contains(tp, "comment") {
			if isTopLevelCommentNode(n, root) {
				startLine := int(n.StartPoint().Row) + 1
				endLine := int(n.EndPoint().Row) + 1
				if n.EndPoint().Column == 0 && endLine > startLine {
					endLine--
				}
				for l := startLine; l <= endLine; l++ {
					lines[l] = true
				}
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func isTopLevelCommentNode(n, root *sitter.Node) bool {
	for parent := n.Parent(); parent != nil; parent = parent.Parent() {
		if parent == root {
			return true
		}
		if parent.Type() != "ERROR" {
			return false
		}
	}
	return false
}

// collectSyntaxOnlyLines finds lines where every leaf node is anonymous
// (punctuation and keywords in tree-sitter grammars). These lines contain
// no meaningful identifiers or literals and don't need doc coverage.
func collectSyntaxOnlyLines(root *sitter.Node, totalLines int) map[int]bool {
	// Track which lines have named leaf nodes (real code).
	hasNamedLeaf := make(map[int]bool)
	// Track which lines have any node at all (to avoid marking empty lines).
	hasAnyNode := make(map[int]bool)

	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if n.ChildCount() == 0 {
			// Leaf node.
			startLine := int(n.StartPoint().Row) + 1
			endLine := int(n.EndPoint().Row) + 1
			if n.EndPoint().Column == 0 && endLine > startLine {
				endLine--
			}
			for l := startLine; l <= endLine; l++ {
				hasAnyNode[l] = true
				if n.IsNamed() {
					hasNamedLeaf[l] = true
				}
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(root)

	result := make(map[int]bool)
	for l := 1; l <= totalLines; l++ {
		if hasAnyNode[l] && !hasNamedLeaf[l] {
			result[l] = true
		}
	}
	return result
}

func collectSkippableLines(node *sitter.Node) map[int]bool {
	lines := make(map[int]bool)
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		tp := n.Type()
		if skippableNodeTypes[tp] || strings.Contains(tp, "comment") {
			startLine := int(n.StartPoint().Row) + 1
			endLine := int(n.EndPoint().Row) + 1
			// If the node ends at column 0 of a line, it doesn't actually
			// occupy that line (trailing newline in the node content).
			if n.EndPoint().Column == 0 && endLine > startLine {
				endLine--
			}
			for l := startLine; l <= endLine; l++ {
				lines[l] = true
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return lines
}
