# Quick Start

## Ways to Run

### 1. Run via `go run` (for development)

**Import bookmarks:**
```bash
go run ./cmd/bookmarks-cli --import ~/bookmarks.html
```

**Run TUI application:**
```bash
go run ./cmd/bookmarks-cli
```

**With custom database path:**
```bash
go run ./cmd/bookmarks-cli --db /path/to/custom.db
```

### 2. Build and Run Binary

**Build:**
```bash
go build -o build/bookmarks-cli ./cmd/bookmarks-cli
```

**Or with a shorter name:**
```bash
go build -o build/bm ./cmd/bookmarks-cli
```

**Run:**
```bash
./build/bookmarks-cli
# or
./build/bm
```

**Import:**
```bash
./build/bookmarks-cli --import ~/bookmarks.html
```

### 3. Install to System (optional)

```bash
# Build and install to $GOPATH/bin or ~/go/bin
go install ./cmd/bookmarks-cli

# Then you can run simply:
bookmarks-cli
```

## Command Line Flags

- `--import <path>` - import bookmarks from HTML file
- `--db <path>` - specify database file path (default: `~/.bookmarks/bookmarks.db`)

## Usage Examples

### Complete Workflow

```bash
# 1. Import bookmarks from browser
go run ./cmd/bookmarks-cli --import ~/Downloads/bookmarks.html

# 2. Run application for viewing and management
go run ./cmd/bookmarks-cli

# 3. Use a different database
go run ./cmd/bookmarks-cli --db ./my-bookmarks.db
```

### TUI Hotkeys

**Navigation:**
- `Tab` - switch between folders panel (left) and bookmarks list (center)
- `↑/↓` - navigate through list/tree
- `Enter` - open selected bookmark in browser / select folder in tree

**Search and Filtering:**
- `/` - start search through bookmarks
- Select folder in tree - show only bookmarks from this folder
- Select "All Bookmarks" at tree root - show all bookmarks

**Management:**
- `a` - add new bookmark
- `e` - edit current bookmark
- `d` - delete current bookmark
- `q` - quit application
- `Esc` - cancel search / close form

## Database Location

By default, the database is created at:
```
~/.bookmarks/bookmarks.db
```

The directory is created automatically on first run.
