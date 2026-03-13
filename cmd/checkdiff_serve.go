package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/renderdocs"
)

//go:embed assets/checkdiff-head.html
var checkdiffHeadHTML string

//go:embed assets/checkdiff.css
var checkdiffCSS string

//go:embed assets/checkdiff.js
var checkdiffJS string

type reviewDoc struct {
	DocPath string   `json:"docPath"`
	Lines   []string `json:"lines"`
}

type reviewEvent struct {
	Type            string `json:"type"`
	DocPath         string `json:"docPath"`
	Line            int    `json:"line,omitempty"`
	Text            string `json:"text,omitempty"`
	ContextBefore   string `json:"contextBefore,omitempty"`
	ContextAfter    string `json:"contextAfter,omitempty"`
	SourceFile      string `json:"sourceFile,omitempty"`
	SourceStartLine int    `json:"sourceStartLine,omitempty"`
	SourceEndLine   int    `json:"sourceEndLine,omitempty"`
	Comment         string `json:"comment,omitempty"`
	Summary         string `json:"summary,omitempty"`
}

var errReviewClosed = errors.New("review already submitted")

type reviewRecorder struct {
	mu          sync.Mutex
	stderr      io.Writer
	initialized map[string]bool
	closed      map[string]bool
	sessionID   string
}

func newReviewRecorder(stderr io.Writer) *reviewRecorder {
	if stderr == nil {
		stderr = os.Stderr
	}
	return &reviewRecorder{
		stderr:      stderr,
		initialized: make(map[string]bool),
		closed:      make(map[string]bool),
		sessionID:   time.Now().Format(time.RFC3339),
	}
}

func (r *reviewRecorder) Record(event reviewEvent) error {
	if strings.TrimSpace(event.DocPath) == "" {
		return errors.New("docPath is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed[event.DocPath] {
		return errReviewClosed
	}

	entry := formatReviewEventMarkdown(event)
	if err := r.appendLocked(event.DocPath, entry); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(r.stderr, strings.TrimRight(entry, "\n")); err != nil {
		return err
	}
	if event.Type == "done" {
		r.closed[event.DocPath] = true
	}
	return nil
}

func (r *reviewRecorder) appendLocked(docPath, entry string) error {
	path := reviewCommentsPath(docPath)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if !r.initialized[path] {
		if _, err := fmt.Fprintf(f, "# Review comments for %s\n\n_Session started %s._\n\n", filepath.Base(docPath), r.sessionID); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
		r.initialized[path] = true
	}
	if _, err := io.WriteString(f, entry); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func serveCheckdiffDocs(docPatterns, sourceDirs []string) error {
	docMatches, err := collectCheckdiffDocMatches(docPatterns)
	if err != nil {
		return err
	}
	if len(docMatches) == 0 {
		return fmt.Errorf("no markdown files matched --docs")
	}

	root := commonAncestorDir(docMatches)
	outDir, err := os.MkdirTemp("", "litcode-checkdiff-*")
	if err != nil {
		return fmt.Errorf("creating temp output dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(outDir)
	}()

	firstRel, err := renderCheckdiffDocs(docMatches, root, outDir, sourceDirs)
	if err != nil {
		return err
	}

	doneCh := make(chan reviewEvent, 1)
	recorder := newReviewRecorder(os.Stderr)
	handler := newCheckdiffServeHandler(outDir, recorder, doneCh)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting local server: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	server := &http.Server{Handler: handler}
	serverErr := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	targetURL := renderedDocURL(listener.Addr().String(), firstRel)
	if err := openBrowser(targetURL); err != nil {
		_ = server.Shutdown(context.Background())
		return err
	}

	fmt.Printf("Serving rendered docs at %s\n", targetURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("serving rendered docs: %w", err)
		}
	case <-ctx.Done():
	case <-doneCh:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stopping local server: %w", err)
	}
	return nil
}

func newCheckdiffServeHandler(outDir string, recorder *reviewRecorder, doneCh chan<- reviewEvent) http.Handler {
	fileHandler := http.FileServer(http.Dir(outDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__litcode/checkdiff.css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = w.Write([]byte(checkdiffCSS))
			return
		case "/__litcode/checkdiff.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			_, _ = w.Write([]byte(checkdiffJS))
			return
		case "/__litcode/comment":
			handleReviewEvent(w, r, recorder, nil)
			return
		case "/__litcode/done":
			handleReviewEvent(w, r, recorder, doneCh)
			return
		}

		if strings.HasSuffix(r.URL.Path, ".html") || r.URL.Path == "/" {
			serveInjectedHTML(w, r, outDir)
			return
		}

		fileHandler.ServeHTTP(w, r)
	})
}

func handleReviewEvent(w http.ResponseWriter, r *http.Request, recorder *reviewRecorder, doneCh chan<- reviewEvent) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	var event reviewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if event.Type == "" {
		if doneCh != nil {
			event.Type = "done"
		} else {
			event.Type = "comment"
		}
	}
	if err := recorder.Record(event); err != nil {
		switch {
		case errors.Is(err, errReviewClosed):
			http.Error(w, err.Error(), http.StatusConflict)
		case err.Error() == "docPath is required":
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	if doneCh != nil {
		select {
		case doneCh <- event:
		default:
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func reviewCommentsPath(docPath string) string {
	if strings.HasSuffix(docPath, ".md") {
		return strings.TrimSuffix(docPath, ".md") + "-comments.md"
	}
	return docPath + "-comments.md"
}

func formatReviewEventMarkdown(event reviewEvent) string {
	var b strings.Builder
	switch event.Type {
	case "done":
		b.WriteString("## Final summary\n\n")
		b.WriteString("- Document: `")
		b.WriteString(event.DocPath)
		b.WriteString("`\n\n")
		if strings.TrimSpace(event.Summary) == "" {
			b.WriteString("_No summary provided._\n\n")
		} else {
			b.WriteString("Summary:\n")
			b.WriteString(quoteMarkdown(event.Summary))
			b.WriteString("\n")
		}
	default:
		b.WriteString("## Comment\n\n")
		b.WriteString("- Document: `")
		b.WriteString(event.DocPath)
		b.WriteString("`\n")
		if event.SourceFile != "" {
			b.WriteString("- Source: `")
			b.WriteString(event.SourceFile)
			b.WriteString("`")
			if span := sourceLineSpan(event.SourceStartLine, event.SourceEndLine); span != "" {
				b.WriteString(" ")
				b.WriteString(span)
			}
			b.WriteString("\n")
		}
		b.WriteString("\nContext before:\n")
		b.WriteString(quoteMarkdown(event.ContextBefore))
		b.WriteString("\nSelection:\n")
		b.WriteString(quoteMarkdown(event.Text))
		b.WriteString("\nContext after:\n")
		b.WriteString(quoteMarkdown(event.ContextAfter))
		b.WriteString("\nComment:\n")
		b.WriteString(quoteMarkdown(event.Comment))
		b.WriteString("\n")
	}
	return b.String()
}

func quoteMarkdown(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "> _none_\n"
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("> ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func sourceLineSpan(start, end int) string {
	switch {
	case start > 0 && end > start:
		return fmt.Sprintf("lines %d-%d", start, end)
	case start > 0:
		return fmt.Sprintf("line %d", start)
	default:
		return ""
	}
}

func serveInjectedHTML(w http.ResponseWriter, r *http.Request, outDir string) {
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if rel == "" {
		rel = "index.html"
	}
	fullPath := filepath.Join(outDir, filepath.FromSlash(rel))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(injectCheckdiffServeHead(data))
}

func injectCheckdiffServeHead(page []byte) []byte {
	html := string(page)
	idx := strings.LastIndex(html, "</head>")
	if idx < 0 {
		return append(page, []byte(checkdiffHeadHTML)...)
	}
	return []byte(html[:idx] + checkdiffHeadHTML + html[idx:])
}

func collectCheckdiffDocMatches(docPatterns []string) ([]filematch.Match, error) {
	matches, err := filematch.Collect(docPatterns, func(relPath string) bool {
		return strings.HasSuffix(relPath, ".md")
	})
	if err != nil {
		return nil, fmt.Errorf("collecting docs: %w", err)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].AbsPath < matches[j].AbsPath
	})
	return matches, nil
}

func renderCheckdiffDocs(matches []filematch.Match, root, outDir string, sourceDirs []string) (string, error) {
	var firstRel string
	for i, match := range matches {
		rel, err := filepath.Rel(root, match.AbsPath)
		if err != nil {
			return "", fmt.Errorf("computing render path for %s: %w", match.AbsPath, err)
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "../") {
			rel = filepath.Base(match.AbsPath)
		}
		if i == 0 {
			firstRel = rel
		}
		if err := renderdocs.RenderFile(match.AbsPath, rel, outDir, sourceDirs, nil); err != nil {
			return "", err
		}
		if err := writeReviewJSON(match.AbsPath, outDir, rel); err != nil {
			return "", err
		}
	}
	return firstRel, nil
}

func writeReviewJSON(srcPath, outDir, rel string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}
	review := reviewDoc{
		DocPath: srcPath,
		Lines:   strings.Split(string(data), "\n"),
	}
	encoded, err := json.Marshal(review)
	if err != nil {
		return fmt.Errorf("encoding review data for %s: %w", srcPath, err)
	}
	reviewPath := filepath.Join(outDir, strings.TrimSuffix(filepath.FromSlash(rel), ".md")+".review.json")
	if err := os.MkdirAll(filepath.Dir(reviewPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(reviewPath, encoded, 0o644); err != nil {
		return err
	}
	return nil
}

func commonAncestorDir(matches []filematch.Match) string {
	if len(matches) == 0 {
		return "."
	}
	root := filepath.Dir(matches[0].AbsPath)
	for _, match := range matches[1:] {
		root = trimToCommonAncestor(root, match.AbsPath)
	}
	return root
}

func trimToCommonAncestor(root, candidate string) string {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	for {
		if candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator)) {
			return root
		}
		next := filepath.Dir(root)
		if next == root {
			return root
		}
		root = next
	}
}

func renderedDocURL(host, rel string) string {
	path := "/" + strings.TrimSuffix(filepath.ToSlash(rel), ".md") + ".html"
	return (&url.URL{
		Scheme: "http",
		Host:   host,
		Path:   path,
	}).String()
}

func openBrowser(targetURL string) error {
	opener := "xdg-open"
	if _, err := exec.LookPath(opener); err != nil {
		if _, fallbackErr := exec.LookPath("open"); fallbackErr == nil {
			opener = "open"
		} else {
			return fmt.Errorf("finding xdg-open: %w", err)
		}
	}
	if err := exec.Command(opener, targetURL).Start(); err != nil {
		return fmt.Errorf("launching %s: %w", opener, err)
	}
	return nil
}
