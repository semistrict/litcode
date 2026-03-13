package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semistrict/litcode/internal/filematch"
)

func TestInjectCheckdiffServeHead(t *testing.T) {
	page := []byte("<html><head><title>x</title></head><body>ok</body></html>")
	injected := string(injectCheckdiffServeHead(page))
	if !strings.Contains(injected, `/__litcode/checkdiff.css`) {
		t.Fatalf("expected css asset in injected head, got:\n%s", injected)
	}
	if !strings.Contains(injected, `/__litcode/checkdiff.js`) {
		t.Fatalf("expected js asset in injected head, got:\n%s", injected)
	}
}

func TestCheckdiffReviewAssets_UseSelectionCommenting(t *testing.T) {
	if strings.Contains(checkdiffJS, "window.prompt") {
		t.Fatalf("checkdiffJS should not use window.prompt:\n%s", checkdiffJS)
	}
	if strings.Contains(checkdiffJS, "window.alert") {
		t.Fatalf("checkdiffJS should not use window.alert:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffJS, `document.createElement("dialog")`) {
		t.Fatalf("checkdiffJS should build a dialog modal:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffJS, `.showModal()`) {
		t.Fatalf("checkdiffJS should open the dialog modal with showModal:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffJS, `"selectionchange"`) {
		t.Fatalf("checkdiffJS should react to text selection changes:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffJS, `window.getSelection()`) {
		t.Fatalf("checkdiffJS should inspect the page selection:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffJS, `litcode-review-selection-button`) {
		t.Fatalf("checkdiffJS should create a floating selection comment button:\n%s", checkdiffJS)
	}
	if !strings.Contains(checkdiffCSS, ".litcode-review-modal") {
		t.Fatalf("checkdiffCSS should include modal styles:\n%s", checkdiffCSS)
	}
	if !strings.Contains(checkdiffCSS, ".litcode-review-selection-button") {
		t.Fatalf("checkdiffCSS should style the floating selection button:\n%s", checkdiffCSS)
	}
}

func TestWriteReviewJSON(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "change.md")
	outDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeReviewJSON(src, outDir, "change.md"); err != nil {
		t.Fatalf("writeReviewJSON: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "change.review.json"))
	if err != nil {
		t.Fatalf("ReadFile(review): %v", err)
	}
	var review reviewDoc
	if err := json.Unmarshal(data, &review); err != nil {
		t.Fatalf("Unmarshal(review): %v", err)
	}
	if review.DocPath != src {
		t.Fatalf("DocPath = %q, want %q", review.DocPath, src)
	}
	if len(review.Lines) != 3 || review.Lines[0] != "first" || review.Lines[1] != "second" {
		t.Fatalf("unexpected review lines: %+v", review.Lines)
	}
}

func TestNewCheckdiffServeHandler_Endpoints(t *testing.T) {
	tmp := t.TempDir()
	htmlPath := filepath.Join(tmp, "change.html")
	if err := os.WriteFile(htmlPath, []byte("<html><head></head><body>ok</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	doneCh := make(chan reviewEvent, 1)
	recorder := newReviewRecorder(&stderr)
	handler := newCheckdiffServeHandler(tmp, recorder, doneCh)

	commentReq := httptest.NewRequest(http.MethodPost, "/__litcode/comment", strings.NewReader(`{"docPath":"`+filepath.Join(tmp, `doc.md`)+`","text":"x","comment":"note","contextBefore":"before","contextAfter":"after","sourceFile":"src/example.go","sourceStartLine":3,"sourceEndLine":5}`))
	commentReq.Header.Set("Content-Type", "application/json")
	commentRec := httptest.NewRecorder()
	handler.ServeHTTP(commentRec, commentReq)
	if commentRec.Code != http.StatusNoContent {
		t.Fatalf("comment status = %d", commentRec.Code)
	}
	if !strings.Contains(stderr.String(), "## Comment") {
		t.Fatalf("expected comment to be written to stderr, got:\n%s", stderr.String())
	}
	commentFile := reviewCommentsPath(filepath.Join(tmp, "doc.md"))
	data, err := os.ReadFile(commentFile)
	if err != nil {
		t.Fatalf("ReadFile(commentFile): %v", err)
	}
	if !strings.Contains(string(data), "src/example.go") {
		t.Fatalf("expected source context in comment file, got:\n%s", string(data))
	}

	doneReq := httptest.NewRequest(http.MethodPost, "/__litcode/done", strings.NewReader(`{"docPath":"`+filepath.Join(tmp, `doc.md`)+`","summary":"ship it"}`))
	doneReq.Header.Set("Content-Type", "application/json")
	doneRec := httptest.NewRecorder()
	handler.ServeHTTP(doneRec, doneReq)
	if doneRec.Code != http.StatusNoContent {
		t.Fatalf("done status = %d", doneRec.Code)
	}

	lateCommentReq := httptest.NewRequest(http.MethodPost, "/__litcode/comment", strings.NewReader(`{"docPath":"`+filepath.Join(tmp, `doc.md`)+`","text":"y","comment":"too late"}`))
	lateCommentReq.Header.Set("Content-Type", "application/json")
	lateCommentRec := httptest.NewRecorder()
	handler.ServeHTTP(lateCommentRec, lateCommentReq)
	if lateCommentRec.Code != http.StatusConflict {
		t.Fatalf("late comment status = %d, want %d", lateCommentRec.Code, http.StatusConflict)
	}

	select {
	case event := <-doneCh:
		if event.Type != "done" || event.Summary != "ship it" {
			t.Fatalf("unexpected doneCh event: %+v", event)
		}
	default:
		t.Fatal("expected done event on channel")
	}

	req := httptest.NewRequest(http.MethodGet, "/change.html", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /change.html status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `/__litcode/checkdiff.js`) {
		t.Fatalf("expected injected js asset, got:\n%s", rec.Body.String())
	}
}

func TestRenderCheckdiffDocs_EndToEndReviewFlow(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	srcDir := filepath.Join(root, "src")
	outDir := filepath.Join(root, "out")

	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	source := "package example\n\nfunc greet() string {\n\treturn \"hi\"\n}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := "# Review\n\n```go file=src/example.go lines=3-5\nfunc greet() string {\n\treturn \"hi\"\n}\n```\n"
	docPath := filepath.Join(docsDir, "change.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	firstRel, err := renderCheckdiffDocs([]filematch.Match{{
		AbsPath: docPath,
		RelPath: "docs/change.md",
	}}, docsDir, outDir, []string{"src/**/*.go"})
	if err != nil {
		t.Fatalf("renderCheckdiffDocs: %v", err)
	}
	if firstRel != "change.md" {
		t.Fatalf("firstRel = %q, want change.md", firstRel)
	}

	var stderr bytes.Buffer
	doneCh := make(chan reviewEvent, 1)
	recorder := newReviewRecorder(&stderr)
	server := httptest.NewServer(newCheckdiffServeHandler(outDir, recorder, doneCh))
	defer server.Close()

	resp, err := http.Get(server.URL + "/change.html")
	if err != nil {
		t.Fatalf("GET change.html: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(change.html): %v", err)
	}
	if !strings.Contains(string(page), `/__litcode/checkdiff.css`) {
		t.Fatalf("expected injected css asset, got:\n%s", string(page))
	}

	reviewResp, err := http.Get(server.URL + "/change.review.json")
	if err != nil {
		t.Fatalf("GET change.review.json: %v", err)
	}
	defer func() {
		_ = reviewResp.Body.Close()
	}()
	var review reviewDoc
	if err := json.NewDecoder(reviewResp.Body).Decode(&review); err != nil {
		t.Fatalf("Decode(review json): %v", err)
	}
	if review.DocPath != docPath {
		t.Fatalf("review.DocPath = %q, want %q", review.DocPath, docPath)
	}
	if len(review.Lines) == 0 || review.Lines[0] != "# Review" {
		t.Fatalf("unexpected review lines: %+v", review.Lines)
	}

	commentResp, err := http.Post(server.URL+"/__litcode/comment", "application/json", strings.NewReader(`{"docPath":"`+docPath+`","text":"# Review","contextBefore":"Title","contextAfter":"body","comment":"Looks good."}`))
	if err != nil {
		t.Fatalf("POST comment: %v", err)
	}
	defer func() {
		_ = commentResp.Body.Close()
	}()
	if commentResp.StatusCode != http.StatusNoContent {
		t.Fatalf("comment status = %d", commentResp.StatusCode)
	}

	doneResp, err := http.Post(server.URL+"/__litcode/done", "application/json", strings.NewReader(`{"docPath":"`+docPath+`","summary":"Finished review."}`))
	if err != nil {
		t.Fatalf("POST done: %v", err)
	}
	defer func() {
		_ = doneResp.Body.Close()
	}()
	if doneResp.StatusCode != http.StatusNoContent {
		t.Fatalf("done status = %d", doneResp.StatusCode)
	}

	if !strings.Contains(stderr.String(), "Finished review.") {
		t.Fatalf("expected stderr to include final summary, got:\n%s", stderr.String())
	}
	commentFile, err := os.ReadFile(reviewCommentsPath(docPath))
	if err != nil {
		t.Fatalf("ReadFile(reviewCommentsPath): %v", err)
	}
	if !strings.Contains(string(commentFile), "Context before:") || !strings.Contains(string(commentFile), "Finished review.") {
		t.Fatalf("expected persisted comments markdown, got:\n%s", string(commentFile))
	}
	select {
	case done := <-doneCh:
		if done.Type != "done" || done.Summary != "Finished review." {
			t.Fatalf("unexpected done event: %+v", done)
		}
	default:
		t.Fatal("expected done event")
	}
}
