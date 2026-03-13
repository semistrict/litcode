package renderdocs

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/semistrict/litcode/internal/expanddocs"
	"github.com/semistrict/litcode/internal/filematch"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

var (
	mdLinkHrefPattern   = regexp.MustCompile(`href="([^"]+)\.md((?:#[^"]*)?)"`)
	mermaidBlockPattern = regexp.MustCompile(`(?s)<pre[^>]*><code[^>]*class="[^"]*\blanguage-mermaid\b[^"]*"[^>]*>(.+?)</code></pre>`)
	chromaCSS           = mustChromaCSS()
	pageTemplate        = template.Must(template.New("page").Parse(`<!doctype html>
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
      --border: #2b3645;
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

	src, err := expanddocs.ExpandedMarkdown(srcPath, sourceDirs)
	if err != nil {
		return fmt.Errorf("expand %s: %w", srcPath, err)
	}

	page, err := renderPage(renderer, filepath.Base(srcPath), src)
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

func renderPage(renderer goldmark.Markdown, filename string, src []byte) ([]byte, error) {
	var body bytes.Buffer
	if err := renderer.Convert(src, &body); err != nil {
		return nil, err
	}

	rendered := rewriteMarkdownLinks(body.String())
	rendered = rewriteMermaidBlocks(rendered)

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
