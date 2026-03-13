package renderdocs

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"html/template"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/semistrict/litcode/internal/expanddocs"
	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/markdown"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var (
	mdLinkHrefPattern      = regexp.MustCompile(`href="([^"]+)\.md((?:#[^"]*)?)"`)
	mermaidBlockPattern    = regexp.MustCompile(`(?s)<pre[^>]*><code[^>]*class="[^"]*\blanguage-mermaid\b[^"]*"[^>]*>(.+?)</code></pre>`)
	sourceFenceInfoPattern = regexp.MustCompile("^```(\\w+)\\s+file=(\\S+)(?:\\s+lines=(\\d+)(?:-(\\d+))?)?$")
	sourceMarkerPattern    = regexp.MustCompile(`<!--litcode-block:(\d+)-->`)
	sourceBlockPattern     = regexp.MustCompile(`(?s)<!--litcode-block:(\d+)-->\s*(<pre\b.*?</pre>|<div class="mermaid">.*?</div>)`)
	chromaCSS              = mustChromaCSS()
	pageTemplate           = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
  <style>
{{ .ChromaCSS }}
    :root {
      color-scheme: dark;
      --bg: #11151c;
      --panel: #171d26;
      --text: #edf2f7;
      --muted: #9aa7b8;
      --accent: #7bdff2;
      --accent-strong: #9bf6ff;
      --border: #2b3645;
      --border-strong: rgba(123, 223, 242, 0.28);
      --code-bg: #202938;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background:
        radial-gradient(circle at top left, rgba(123, 223, 242, 0.16), transparent 28rem),
        linear-gradient(180deg, #151b24 0%, var(--bg) 100%);
      color: var(--text);
      font-family: Georgia, "Iowan Old Style", "Palatino Linotype", serif;
      line-height: 1.65;
    }
    main {
      max-width: 54rem;
      margin: 0 auto;
      padding: 3rem 1.5rem 5rem;
    }
    article {
      background: rgba(23, 29, 38, 0.92);
      border: 1px solid var(--border);
      border-radius: 18px;
      padding: 2rem;
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.28);
      backdrop-filter: blur(8px);
    }
    h1, h2, h3 {
      font-family: "Avenir Next Condensed", "Franklin Gothic Medium", sans-serif;
      line-height: 1.1;
      letter-spacing: 0.02em;
    }
    h1 { font-size: clamp(2.5rem, 5vw, 3.6rem); margin-top: 0; }
    h2 { font-size: 1.8rem; margin-top: 2.4rem; }
    h3 { font-size: 1.25rem; margin-top: 1.8rem; }
    p, li { font-size: 1.05rem; }
    a { color: var(--accent); }
    code {
      font-family: "SFMono-Regular", Menlo, Consolas, monospace;
      background: var(--code-bg);
      padding: 0.1rem 0.3rem;
      border-radius: 0.3rem;
    }
    pre {
      overflow-x: auto;
      padding: 1rem;
      border-radius: 12px;
      background: #1e1f24;
      color: #f7f4ef;
    }
    pre.tok-chroma {
      background: #1e1f24;
      color: #f7f4ef;
    }
    pre code {
      background: transparent;
      padding: 0;
      border-radius: 0;
      color: inherit;
    }
    .litcode-block {
      margin: 1.6rem 0;
    }
    .litcode-block > pre,
    .litcode-block > .mermaid {
      margin: 0;
    }
    .litcode-block-toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 0.8rem;
      padding: 0.8rem 0.95rem;
      border: 1px solid var(--border-strong);
      border-bottom: 0;
      border-radius: 14px 14px 0 0;
      background:
        linear-gradient(180deg, rgba(123, 223, 242, 0.12), rgba(123, 223, 242, 0.03)),
        rgba(23, 29, 38, 0.86);
    }
    .litcode-block-toolbar + pre,
    .litcode-block-toolbar + .mermaid {
      border-top-left-radius: 0;
      border-top-right-radius: 0;
    }
    .litcode-block-label {
      min-width: 0;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0.55rem 0.8rem;
      font-family: "Avenir Next", "Helvetica Neue", sans-serif;
      letter-spacing: 0.02em;
    }
    .litcode-block-file {
      font-family: "SFMono-Regular", Menlo, Consolas, monospace;
      font-size: 0.85rem;
      color: var(--text);
      overflow-wrap: anywhere;
    }
    .litcode-block-lines {
      font-size: 0.78rem;
      color: var(--muted);
      text-transform: uppercase;
    }
    .litcode-block-actions {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      flex-wrap: wrap;
      justify-content: flex-end;
    }
    .litcode-link {
      border: 1px solid rgba(123, 223, 242, 0.28);
      border-radius: 999px;
      padding: 0.38rem 0.75rem;
      background: rgba(123, 223, 242, 0.1);
      color: var(--accent-strong);
      font-family: "Avenir Next", "Helvetica Neue", sans-serif;
      font-size: 0.82rem;
      line-height: 1;
      letter-spacing: 0.02em;
      text-decoration: none;
      transition:
        transform 180ms ease,
        border-color 180ms ease,
        background-color 180ms ease,
        color 180ms ease;
    }
    .litcode-link:hover,
    .litcode-link:focus-visible {
      transform: translateY(-1px);
      border-color: rgba(155, 246, 255, 0.5);
      background: rgba(123, 223, 242, 0.16);
      color: #c9fbff;
      outline: none;
    }
    .mermaid {
      margin: 1.5rem 0;
      padding: 1rem;
      border: 1px solid var(--border);
      border-radius: 12px;
      background: var(--panel);
    }
    @media (max-width: 700px) {
      article { padding: 1.25rem; }
      main { padding: 1rem 0.75rem 3rem; }
      .litcode-block-toolbar {
        align-items: flex-start;
        flex-direction: column;
      }
      .litcode-block-actions {
        justify-content: flex-start;
      }
    }
  </style>
</head>
<body>
  <main>
    <article>
{{ .Body }}
    </article>
  </main>
  <script type="module">
    import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs";
    mermaid.initialize({ startOnLoad: true, theme: "dark" });
  </script>
</body>
</html>
`))
)

type sourceBlockMeta struct {
	File      string
	GitHubURL string
	StartLine int
	EndLine   int
}

func RenderFile(srcPath, relPath, outDir string, sourceDirs []string, out io.Writer) error {
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return err
	}

	outPath := filepath.Join(absOut, strings.TrimSuffix(filepath.FromSlash(relPath), ".md")+".html")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	renderer := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-mocha"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.ClassPrefix("tok-"),
				),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)

	sourceIndex, err := filematch.Index(sourceDirs)
	if err != nil {
		return fmt.Errorf("collecting source files: %w", err)
	}
	blocks, err := markdown.ParseFile(srcPath)
	if err != nil {
		return err
	}

	src, err := expanddocs.ExpandedMarkdown(srcPath, sourceDirs)
	if err != nil {
		return fmt.Errorf("expand %s: %w", srcPath, err)
	}

	annotated, sourceBlockMetas, err := annotateSourceFences(src, blocks, sourceIndex, sourceDirs, detectGitHubBlobBase())
	if err != nil {
		return fmt.Errorf("annotate source blocks for %s: %w", srcPath, err)
	}

	page, err := renderPage(renderer, filepath.Base(srcPath), annotated, sourceBlockMetas)
	if err != nil {
		return fmt.Errorf("render %s: %w", srcPath, err)
	}
	if err := os.WriteFile(outPath, page, 0o644); err != nil {
		return err
	}
	if out != nil {
		displayPath := outPath
		if relOut, err := filepath.Rel(cwd, outPath); err == nil && relOut != "" && relOut != "." && !strings.HasPrefix(relOut, ".."+string(os.PathSeparator)) && relOut != ".." {
			displayPath = relOut
		}
		_, _ = fmt.Fprintln(out, displayPath)
	}
	return nil
}

// RenderTree renders all markdown files under srcDir to HTML under outDir,
// preserving the relative directory structure and replacing .md with .html.
func RenderTree(srcDir, outDir string, sourceDirs []string, out io.Writer) error {
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	docMatches, err := filematch.Collect([]string{srcDir}, func(relPath string) bool {
		return strings.HasSuffix(relPath, ".md")
	})
	if err != nil {
		return err
	}
	for _, match := range docMatches {
		relPath, err := filepath.Rel(absSrc, match.AbsPath)
		if err != nil {
			return err
		}
		if err := RenderFile(match.AbsPath, filepath.ToSlash(relPath), outDir, sourceDirs, out); err != nil {
			return err
		}
	}
	return nil
}

func renderPage(renderer goldmark.Markdown, filename string, src []byte, blocks []sourceBlockMeta) ([]byte, error) {
	var body bytes.Buffer
	if err := renderer.Convert(src, &body); err != nil {
		return nil, err
	}

	rendered := rewriteMarkdownLinks(body.String())
	rendered = rewriteMermaidBlocks(rendered)
	rendered = decorateSourceBlocks(rendered, blocks)

	var page bytes.Buffer
	data := struct {
		Title     string
		Body      template.HTML
		ChromaCSS template.CSS
	}{
		Title:     documentTitle(filename, src),
		Body:      template.HTML(rendered),
		ChromaCSS: chromaCSS,
	}
	if err := pageTemplate.Execute(&page, data); err != nil {
		return nil, err
	}
	return page.Bytes(), nil
}

func rewriteMarkdownLinks(s string) string {
	return mdLinkHrefPattern.ReplaceAllString(s, `href="$1.html$2"`)
}

func rewriteMermaidBlocks(s string) string {
	return mermaidBlockPattern.ReplaceAllStringFunc(s, func(block string) string {
		m := mermaidBlockPattern.FindStringSubmatch(block)
		if len(m) != 2 {
			return block
		}
		return `<div class="mermaid">` + stdhtml.UnescapeString(m[1]) + `</div>`
	})
}

func decorateSourceBlocks(rendered string, blocks []sourceBlockMeta) string {
	decorated := sourceBlockPattern.ReplaceAllStringFunc(rendered, func(match string) string {
		parts := sourceBlockPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil || index < 0 || index >= len(blocks) {
			return parts[2]
		}
		return buildSourceBlockHTML(blocks[index], parts[2])
	})
	return sourceMarkerPattern.ReplaceAllString(decorated, "")
}

func buildSourceBlockHTML(meta sourceBlockMeta, blockHTML string) string {
	fileLabel := stdhtml.EscapeString(meta.File)
	var lineLabel string
	switch {
	case meta.StartLine > 0 && meta.EndLine > meta.StartLine:
		lineLabel = fmt.Sprintf("Lines %d-%d", meta.StartLine, meta.EndLine)
	case meta.StartLine > 0:
		lineLabel = fmt.Sprintf("Line %d", meta.StartLine)
	default:
		lineLabel = "Source-linked block"
	}

	var b strings.Builder
	b.WriteString(`<div class="litcode-block">`)
	b.WriteString(`<div class="litcode-block-toolbar">`)
	b.WriteString(`<div class="litcode-block-label">`)
	b.WriteString(`<span class="litcode-block-file">`)
	b.WriteString(fileLabel)
	b.WriteString(`</span>`)
	b.WriteString(`<span class="litcode-block-lines">`)
	b.WriteString(stdhtml.EscapeString(lineLabel))
	b.WriteString(`</span>`)
	b.WriteString(`</div>`)
	if meta.GitHubURL != "" {
		b.WriteString(`<div class="litcode-block-actions">`)
		b.WriteString(`<a class="litcode-link" href="`)
		b.WriteString(stdhtml.EscapeString(meta.GitHubURL))
		b.WriteString(`" target="_blank" rel="noreferrer noopener">GitHub</a>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(blockHTML)
	b.WriteString(`</div>`)
	return b.String()
}

func annotateSourceFences(src []byte, blocks []markdown.CodeBlock, sourceIndex map[string]string, sourceDirs []string, gitHubBlobBase string) ([]byte, []sourceBlockMeta, error) {
	lines := strings.Split(string(src), "\n")
	metaByIndex := make([]sourceBlockMeta, 0, len(blocks))
	linesCache := make(map[string][]string)
	blockIndex := 0

	var out []string
	for _, line := range lines {
		if blockIndex < len(blocks) {
			if info, ok := parseSourceFenceLine(line); ok && sameFenceBlock(info, blocks[blockIndex]) {
				meta, err := sourceBlockMetadata(blocks[blockIndex], sourceIndex, sourceDirs, linesCache, gitHubBlobBase)
				if err != nil {
					return nil, nil, err
				}
				out = append(out, fmt.Sprintf("<!--litcode-block:%d-->", len(metaByIndex)))
				metaByIndex = append(metaByIndex, meta)
				blockIndex++
			}
		}
		out = append(out, line)
	}

	return []byte(strings.Join(out, "\n")), metaByIndex, nil
}

func sourceBlockMetadata(block markdown.CodeBlock, sourceIndex map[string]string, sourceDirs []string, linesCache map[string][]string, gitHubBlobBase string) (sourceBlockMeta, error) {
	absPath, found := resolveSourceFile(block.File, sourceIndex)
	if !found {
		return sourceBlockMeta{}, fmt.Errorf("%s:%d: file not found in any source directory: %s", block.DocFile, block.DocLine, block.File)
	}

	srcLines, ok := linesCache[absPath]
	if !ok {
		var err error
		srcLines, err = readLines(absPath)
		if err != nil {
			return sourceBlockMeta{}, fmt.Errorf("%s:%d: cannot read source file %s: %w", block.DocFile, block.DocLine, block.File, err)
		}
		linesCache[absPath] = srcLines
	}

	repoRel := sourceRepoRelPath(absPath, sourceDirs)
	startLine, endLine := resolveBlockLineRange(block, srcLines)
	return sourceBlockMeta{
		File:      repoRel,
		GitHubURL: githubLineURL(gitHubBlobBase, repoRel, startLine, endLine),
		StartLine: startLine,
		EndLine:   endLine,
	}, nil
}

func resolveSourceFile(file string, sourceIndex map[string]string) (string, bool) {
	path, ok := sourceIndex[filepath.ToSlash(filepath.Clean(file))]
	return path, ok
}

func sourceRepoRelPath(absPath string, sourceDirs []string) string {
	var candidates []string
	cwd, _ := os.Getwd()

	for _, source := range sourceDirs {
		if hasMeta(source) {
			if cwd != "" {
				if rel, err := filepath.Rel(cwd, absPath); err == nil {
					candidates = append(candidates, filepath.ToSlash(filepath.Clean(rel)))
				}
			}
			continue
		}

		info, err := os.Stat(source)
		if err != nil {
			continue
		}
		absSource, err := filepath.Abs(source)
		if err != nil {
			continue
		}
		if info.IsDir() {
			rel, err := filepath.Rel(absSource, absPath)
			if err == nil {
				rel = filepath.ToSlash(filepath.Clean(rel))
				if rel != "." && rel != ".." && !strings.HasPrefix(rel, "../") {
					candidates = append(candidates, rel)
				}
			}
			continue
		}
		if filepath.Clean(absSource) == filepath.Clean(absPath) {
			candidates = append(candidates, filepath.Base(absPath))
		}
	}

	if len(candidates) == 0 {
		return filematch.RelPath(absPath)
	}

	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if len(candidate) < len(best) || (len(candidate) == len(best) && candidate < best) {
			best = candidate
		}
	}
	return best
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func resolveBlockLineRange(block markdown.CodeBlock, srcLines []string) (int, int) {
	if block.StartLine > 0 {
		endLine := block.EndLine
		if endLine < block.StartLine {
			endLine = block.StartLine
		}
		return block.StartLine, endLine
	}

	content := block.Content
	if expanded, abbreviated, err := expanddocs.ExpandAbbreviatedBlock(block, srcLines); err == nil && abbreviated {
		content = expanded
	}
	if len(content) == 0 {
		return 0, 0
	}

	matches := findAllContent(content, srcLines)
	if len(matches) == 0 {
		return 0, 0
	}
	startLine := matches[0] + 1
	endLine := startLine + len(content) - 1
	return startLine, endLine
}

func findAllContent(content, srcLines []string) []int {
	if len(content) == 0 || len(content) > len(srcLines) {
		return nil
	}
	normContent := normalizeLines(content)
	normSrc := normalizeLines(srcLines)

	var matches []int
	for i := 0; i <= len(normSrc)-len(normContent); i++ {
		if equalStrings(normContent, normSrc[i:i+len(normContent)]) {
			matches = append(matches, i)
		}
	}
	return matches
}

func normalizeLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = normalizeLine(line)
	}
	return out
}

func normalizeLine(s string) string {
	s = stripTrailingComment(s)
	s = strings.TrimSpace(s)
	var b strings.Builder
	inWhitespace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !inWhitespace {
				b.WriteByte(' ')
				inWhitespace = true
			}
			continue
		}
		b.WriteRune(r)
		inWhitespace = false
	}
	return strings.TrimRight(b.String(), " ")
}

func stripTrailingComment(s string) string {
	inString := rune(0)
	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString != 0 {
			escaped = true
			continue
		}
		if inString != 0 {
			if r == inString {
				inString = 0
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			inString = r
			continue
		}
		if r == '/' && i+1 < len(s) && s[i+1] == '/' {
			return s[:i]
		}
		if r == '#' {
			return s[:i]
		}
	}
	return s
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fenceInfo struct {
	Language  string
	File      string
	StartLine int
	EndLine   int
}

func parseSourceFenceLine(line string) (fenceInfo, bool) {
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 {
		return fenceInfo{}, false
	}
	line = strings.TrimSpace(line)
	m := sourceFenceInfoPattern.FindStringSubmatch(line)
	if len(m) == 0 {
		return fenceInfo{}, false
	}
	info := fenceInfo{
		Language: m[1],
		File:     m[2],
	}
	if m[3] != "" {
		info.StartLine, _ = strconv.Atoi(m[3])
		info.EndLine = info.StartLine
		if m[4] != "" {
			info.EndLine, _ = strconv.Atoi(m[4])
		}
	}
	return info, true
}

func sameFenceBlock(info fenceInfo, block markdown.CodeBlock) bool {
	return info.Language == block.Language &&
		info.File == block.File &&
		info.StartLine == block.StartLine &&
		info.EndLine == block.EndLine
}

func githubLineURL(blobBaseURL, relPath string, startLine, endLine int) string {
	if blobBaseURL == "" {
		return ""
	}
	url := strings.TrimRight(blobBaseURL, "/") + "/" + filepath.ToSlash(relPath)
	switch {
	case startLine > 0 && endLine > startLine:
		return fmt.Sprintf("%s#L%d-L%d", url, startLine, endLine)
	case startLine > 0:
		return fmt.Sprintf("%s#L%d", url, startLine)
	default:
		return url
	}
}

func detectGitHubBlobBase() string {
	revision, err := gitOutput("rev-parse", "HEAD")
	if err != nil || revision == "" {
		return ""
	}
	remote, err := gitOutput("config", "--get", "remote.origin.url")
	if err != nil || remote == "" {
		return ""
	}
	repoURL, ok := githubRepoURL(remote)
	if !ok {
		return ""
	}
	return strings.TrimRight(repoURL, "/") + "/blob/" + revision
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func githubRepoURL(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		remote = strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		remote = strings.TrimPrefix(remote, "ssh://git@github.com/")
	case strings.HasPrefix(remote, "https://github.com/"):
		remote = strings.TrimPrefix(remote, "https://github.com/")
	case strings.HasPrefix(remote, "http://github.com/"):
		remote = strings.TrimPrefix(remote, "http://github.com/")
	default:
		return "", false
	}

	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.Trim(remote, "/")
	if remote == "" {
		return "", false
	}
	return "https://github.com/" + remote, true
}

func documentTitle(filename string, src []byte) string {
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func mustChromaCSS() template.CSS {
	style := styles.Get("catppuccin-mocha")
	if style == nil {
		style = styles.Fallback
	}

	var css bytes.Buffer
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.ClassPrefix("tok-"),
	)
	if err := formatter.WriteCSS(&css, style); err != nil {
		panic(err)
	}
	return template.CSS(css.String())
}

func hasMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}
