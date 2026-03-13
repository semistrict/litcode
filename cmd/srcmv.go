package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/semistrict/litcode/internal/srcmv"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(srcmvCmd)
}

var srcmvCmd = &cobra.Command{
	Use:          "srcmv <src.go:line[:col]> <dest.go:line>",
	Short:        "Move a top-level declaration between files",
	SilenceUsage: true,
	Long: `Moves a top-level declaration (with its doc comments) from one source file
to another. The declaration is identified by file, line, and optional column.

Examples:
  litcode srcmv internal/checker/checker.go:150 internal/checker/matching.go:1
  litcode srcmv foo.go:10:1 bar.go:5`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcFile, srcLine, srcCol, err := parseSrcArg(args[0])
		if err != nil {
			return fmt.Errorf("invalid source: %w", err)
		}
		destFile, destLine, err := parseDestArg(args[1])
		if err != nil {
			return fmt.Errorf("invalid destination: %w", err)
		}

		if err := srcmv.Move(srcFile, srcLine, srcCol, destFile, destLine); err != nil {
			return err
		}

		fmt.Printf("Moved declaration from %s:%d to %s:%d\n", srcFile, srcLine, destFile, destLine)
		return nil
	},
}

// parseSrcArg parses "file:line" or "file:line:col".
func parseSrcArg(arg string) (file string, line, col int, err error) {
	parts := strings.Split(arg, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return "", 0, 0, fmt.Errorf("expected file:line[:col], got %q", arg)
	}
	file = parts[0]
	line, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid line number %q", parts[1])
	}
	if len(parts) == 3 {
		col, err = strconv.Atoi(parts[2])
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid column number %q", parts[2])
		}
	}
	return file, line, col, nil
}

// parseDestArg parses "file:line".
func parseDestArg(arg string) (file string, line int, err error) {
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("expected file:line, got %q", arg)
	}
	file = parts[0]
	line, err = strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid line number %q", parts[1])
	}
	return file, line, nil
}
