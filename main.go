package main

import (
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	log.SetFlags(0)

	bind := flag.String("bind", "", "Specify alternate bind address [default: all interfaces]")
	flag.StringVar(bind, "b", "", "Specify alternate bind address [default: all interfaces]")
	directory := flag.String("directory", ".", "Specify alternative directory [default: current directory]")
	flag.StringVar(directory, "d", ".", "Specify alternative directory [default: current directory]")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [port]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Positional arguments:\n")
		fmt.Fprintf(os.Stderr, "  port        Specify alternate port [default: 8000]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	port := "8000"
	if flag.NArg() > 0 {
		port = flag.Arg(0)
	}

	absDir, err := filepath.Abs(*directory)
	if err != nil {
		log.Fatalf("Error resolving directory: %v", err)
	}

	root, err := os.OpenRoot(absDir)
	if err != nil {
		log.Fatalf("Error opening directory: %v", err)
	}
	defer root.Close()

	handler := &fileServerHandler{rootDir: absDir, root: root}

	addr := net.JoinHostPort(*bind, port)

	listenAddr := *bind
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}

	displayAddr := listenAddr
	if listenAddr == "0.0.0.0" || listenAddr == "::" {
		displayAddr = "localhost"
	}

	fmt.Printf("Serving HTTP on %s port %s (http://%s/) ...\n", listenAddr, port, net.JoinHostPort(displayAddr, port))

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}
	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

type fileServerHandler struct {
	rootDir string
	root    *os.Root
}

func (h *fileServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Log the request in Python http.server format
	clientAddr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(clientAddr); err == nil {
		clientAddr = host
	}

	// Clean the URL path and convert to relative path for os.Root
	urlPath := path.Clean(r.URL.Path)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	relPath := strings.TrimPrefix(urlPath, "/")
	if relPath == "" {
		relPath = "."
	}

	info, err := h.root.Stat(relPath)
	if os.IsNotExist(err) {
		http.Error(w, "404 File not found", http.StatusNotFound)
		logRequest(clientAddr, r, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		logRequest(clientAddr, r, http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		indexRelPath := path.Join(relPath, "index.html")
		if indexInfo, err := h.root.Stat(indexRelPath); err == nil && !indexInfo.IsDir() {
			serveFile(w, r, h.root, indexRelPath, clientAddr)
			return
		}

		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			logRequest(clientAddr, r, http.StatusMovedPermanently)
			return
		}

		serveDirListing(w, r, h.root, relPath, urlPath, clientAddr)
		return
	}

	serveFile(w, r, h.root, relPath, clientAddr)
}

func serveFile(w http.ResponseWriter, r *http.Request, root *os.Root, relPath string, clientAddr string) {
	file, err := root.Open(relPath)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		logRequest(clientAddr, r, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		logRequest(clientAddr, r, http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, path.Base(relPath), info.ModTime(), file)
	logRequest(clientAddr, r, http.StatusOK)
}

func serveDirListing(w http.ResponseWriter, r *http.Request, root *os.Root, relPath string, urlPath string, clientAddr string) {
	dir, err := root.Open(relPath)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		logRequest(clientAddr, r, http.StatusInternalServerError)
		return
	}
	defer dir.Close()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		logRequest(clientAddr, r, http.StatusInternalServerError)
		return
	}

	// Sort entries: directories first, then files, both alphabetically
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].IsDir()
		jIsDir := entries[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	displayPath := urlPath
	if displayPath == "" {
		displayPath = "/"
	}

	fmt.Fprintf(w, `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<title>Directory listing for %s</title>
</head>
<body>
<h1>Directory listing for %s</h1>
<hr>
<ul>
`, html.EscapeString(displayPath), html.EscapeString(displayPath))

	for _, entry := range entries {
		name := entry.Name()
		displayName := name
		if entry.IsDir() {
			displayName = name + "/"
			name = name + "/"
		}
		linkPath := path.Join(urlPath, name)
		fmt.Fprintf(w, `<li><a href="%s">%s</a></li>
`, html.EscapeString(linkPath), html.EscapeString(displayName))
	}

	fmt.Fprintf(w, `</ul>
<hr>
</body>
</html>
`)

	logRequest(clientAddr, r, http.StatusOK)
}

func logRequest(clientAddr string, r *http.Request, statusCode int) {
	// Format similar to Python's http.server:
	// 127.0.0.1 - - [26/Nov/2025 10:30:45] "GET / HTTP/1.1" 200 -
	log.Printf("%s - - \"%s %s %s\" %d -\n",
		clientAddr,
		r.Method,
		r.URL.Path,
		r.Proto,
		statusCode,
	)
}
