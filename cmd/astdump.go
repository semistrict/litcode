package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/semistrict/litcode/internal/comments"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(astDumpCmd)
}

var astDumpCmd = &cobra.Command{
	Use:           "ast-dump <file>",
	Short:         "Dump the tree-sitter AST for a source file",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		lang := comments.LanguageForFilename(path)
		if lang == nil {
			return fmt.Errorf("unsupported file extension: %s", filepath.Ext(path))
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		parser := sitter.NewParser()
		parser.SetLanguage(lang)
		tree, err := parser.ParseCtx(context.Background(), nil, src)
		if err != nil {
			return fmt.Errorf("parse error: %w", err)
		}
		defer tree.Close()

		printNode(tree.RootNode(), src, 0)
		return nil
	},
}

func printNode(n *sitter.Node, src []byte, depth int) {
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
	fmt.Printf("%s%s (%s%s) [%d-%d] %q\n", indent, n.Type(), named, leaf, startRow, endRow, content)
	for i := 0; i < int(n.ChildCount()); i++ {
		printNode(n.Child(i), src, depth+1)
	}
}
