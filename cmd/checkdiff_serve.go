package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/renderdocs"
)

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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting local server: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	server := &http.Server{
		Handler: http.FileServer(http.Dir(outDir)),
	}
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
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("stopping local server: %w", err)
	}
	return nil
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
	}
	return firstRel, nil
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
