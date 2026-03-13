# Tree-sitter Classification

[Back to Overview](overview.md) | Previous: [Markdown Parsing](markdown-parsing.md) | Next: [Types & Configuration](checker-types.md)

Not every source line needs documentation. Comment-only lines, blank lines,
`package` declarations, and `import` statements are boilerplate — requiring
coverage for them would make the documentation tedious and noisy. We use
tree-sitter to identify these lines precisely, rather than relying on fragile
regex heuristics.

## Supported languages

The `extToLang` map connects file extensions to tree-sitter grammar objects.
When a file's extension isn't in this map, all of its lines are treated as
requiring coverage (the conservative default):

```go file=internal/comments/comments.go
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
```

## Which AST nodes are skippable?

The `skippableNodeTypes` map lists tree-sitter node types that don't need
doc coverage. These fall into a few categories:

- **Comments** across all languages: `comment`, `line_comment`, `block_comment`
- **Package declarations**: `package_clause` (Go), `package_declaration` (Java)
- **Import statements**: various names across languages
- **Preprocessor includes**: `preproc_include` (C/C++)
- **Ruby requires**: `require`, `require_relative`

```go file=internal/comments/comments.go
var skippableNodeTypes = map[string]bool{
	"comment":                   true,
	"line_comment":              true,
	"block_comment":             true,
	"package_clause":            true,
	"package_declaration":       true,
	"import_declaration":        true,
	"import_spec":               true,
	"import_spec_list":          true,
	"import_statement":          true,
	"import_from_statement":     true,
	"use_declaration":           true,
	"extern_crate_declaration":  true,
	"preproc_include":           true,
	"require":                   true,
	"require_relative":          true,
}
```

Note that some node types share names across languages (e.g. `import_statement`
is used by both JavaScript and Python). This is fine — the map is keyed by
string, so duplicates just overwrite with the same value.

## Shared parser setup

Both coverage classification and comment-placement warnings need the same
tree-sitter parser setup. `LanguageForFilename` returns the tree-sitter language
for a file based on its extension (or nil if unsupported), and `ParseTree`
returns nil when the language is unsupported or the parse fails:

```go file=internal/comments/comments.go
func LanguageForFilename(filename string) *sitter.Language {
	return extToLang[filepath.Ext(filename)]
}
```

`ParseTree` parses source code using the grammar appropriate for the filename's
extension. It returns nil if the extension is unsupported or parsing fails:

```go file=internal/comments/comments.go
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
```

## The SkippableLines function

This is the main entry point for callers. It takes a filename (for extension
detection) and the file's source bytes, and returns a set of 1-based line
numbers that can be skipped:

```go file=internal/comments/comments.go
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
```

The function returns `nil` (not an empty map) for unsupported extensions. The
checker interprets `nil` as "no classification available — all lines need
coverage."

The same parser helper is also used to find comments that are top-level in a
documentation block. These are comments whose ancestors are only the root node
and optional `ERROR` wrappers, which lets the checker distinguish "split this
block into two examples" comments from ordinary nested implementation comments:

```go file=internal/comments/comments.go
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
```

## Walking the AST

The `collectSkippableLines` helper recursively walks the tree-sitter AST. When
it encounters a node whose type is in the skippable set (or contains the word
"comment"), it marks all lines that node spans:

```go file=internal/comments/comments.go
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
```

There's an important subtlety in the `EndPoint` correction above: some tree-sitter grammars
(notably Rust's `///` doc comments) produce nodes whose `EndPoint` is at
column 0 of the *next* line. Without this correction, the function would
incorrectly mark the line after a doc comment as skippable. We detect this
case by checking if the end column is 0 and adjusting `endLine` down by one.

When a node is skippable, the walk *returns* without visiting children — this
prevents descending into the internal structure of, say, an import block (which
might contain identifiers that aren't themselves skippable node types).

## Syntax-only lines

Not all lines that lack documentation-worthy content are comments or imports.
Some lines contain only syntax — punctuation and keywords like `{`, `}`, or
`else` — with no meaningful identifiers or literals. The `collectSyntaxOnlyLines`
function identifies these by walking to every leaf node in the AST and checking
whether it is *named* (a real code token) or *anonymous* (syntax/punctuation):

```go file=internal/comments/comments.go
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
```

A line is marked syntax-only when it has at least one leaf node (so it isn't
blank) but none of those leaves are *named* nodes. In tree-sitter grammars,
named nodes represent identifiers, literals, and other semantically meaningful
tokens, while anonymous nodes represent punctuation (`{`, `}`, `,`) and
keywords (`else`, `func`). Lines containing only anonymous leaves — like a
lone closing brace — don't need documentation coverage.

Continue to [Types & Configuration](checker-types.md) to see the data types used
by the checker.
