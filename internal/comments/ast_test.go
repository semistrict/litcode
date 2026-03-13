package comments_test

import (
	"context"
	"fmt"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// TestASTDump parses a Go source snippet and dumps the full AST.
// Edit the `src` variable to inspect any code you need to understand.
func TestASTDump(t *testing.T) {
	src := []byte(`package main

func main() {
	if true {
		doSomething()
	} else {
		doOther()
	}
}
`)
	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	dumpNode(t, tree.RootNode(), src, 0)
}

func dumpNode(t *testing.T, n *sitter.Node, src []byte, depth int) {
	t.Helper()
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	named := "named"
	if !n.IsNamed() {
		named = "anon"
	}
	leaf := ""
	if n.ChildCount() == 0 {
		leaf = " LEAF"
	}
	startRow := n.StartPoint().Row + 1
	endRow := n.EndPoint().Row + 1
	content := n.Content(src)
	if len(content) > 80 {
		content = content[:80] + "..."
	}
	t.Logf("%s%s (%s%s) [%d-%d] %q", indent, n.Type(), named, leaf, startRow, endRow, content)
	for i := 0; i < int(n.ChildCount()); i++ {
		dumpNode(t, n.Child(i), src, depth+1)
	}
	_ = fmt.Sprintf("") // avoid unused import
}
