# go-python-http-server

A Go implementation of Python's `python3 -m http.server`. Serves files from the current directory over HTTP.

## Installation

```bash
go install github.com/gonzaloserrano/go-python-http-server@latest
```

## Usage

```bash
# Serve current directory on port 8000
go-python-http-server

# Serve on custom port
go-python-http-server 3000

# Serve specific directory
go-python-http-server -d /path/to/dir

# Bind to specific address
go-python-http-server -b 127.0.0.1 8080
```

Or run directly without installing:

```bash
go run github.com/gonzaloserrano/go-python-http-server@latest 8080
```

## Options

```
Usage: go-python-http-server [options] [port]

Positional arguments:
  port        Specify alternate port [default: 8000]

Options:
  -b, -bind string
        Specify alternate bind address [default: all interfaces]
  -d, -directory string
        Specify alternative directory [default: current directory]
```

## Features

- Serves static files from the specified directory
- Directory listings with HTML
- Automatically serves `index.html` if present
- Request logging in Python http.server format
- No external dependencies
