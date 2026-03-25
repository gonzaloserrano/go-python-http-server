package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestHandler(t *testing.T) *fileServerHandler {
	t.Helper()
	dir := t.TempDir()

	// hello.txt
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello, World!"), 0o644); err != nil {
		t.Fatal(err)
	}

	// subdir/file.txt
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "subdir", "file.txt"), []byte("sub content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// indexdir/index.html
	if err := os.MkdirAll(filepath.Join(dir, "indexdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "indexdir", "index.html"), []byte("<h1>Sub Index</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })

	return &fileServerHandler{rootDir: dir, root: root}
}

func TestServeHTTP(t *testing.T) {
	t.Parallel()
	handler := setupTestHandler(t)

	testCases := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "ServeFile",
			path:           "/hello.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
		},
		{
			name:           "NotFound",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "404 File not found",
		},
		{
			name:           "RootDirectoryListing",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "hello.txt",
		},
		{
			name:           "SubdirListing",
			path:           "/subdir/",
			expectedStatus: http.StatusOK,
			expectedBody:   "file.txt",
		},
		{
			name:           "IndexHTML",
			path:           "/indexdir/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<h1>Sub Index</h1>",
		},
		{
			name:           "TrailingSlashRedirect",
			path:           "/subdir",
			expectedStatus: http.StatusMovedPermanently,
		},
		{
			name:           "PathTraversalDotDot",
			path:           "/../../../etc/passwd",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PathTraversalEncoded",
			path:           "/..%2f..%2f..%2fetc/passwd",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.expectedStatus)
			}

			if tc.expectedBody != "" {
				body, _ := io.ReadAll(rec.Body)
				if got := string(body); !strings.Contains(got, tc.expectedBody) {
					t.Errorf("body = %q, want to contain %q", got, tc.expectedBody)
				}
			}
		})
	}
}

func TestPathTraversalContent(t *testing.T) {
	t.Parallel()
	handler := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "root:") {
		t.Error("response contains /etc/passwd content -- path traversal succeeded")
	}
}
