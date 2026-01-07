# Bookmarks CLI Architecture

## Overview

The project has been reorganized using Clean Architecture principles and Separation of Concerns. This makes the code more maintainable, testable, and extensible.

## Project Structure

```
bookmarks-cli/
├── cmd/
│   └── bookmarks-cli/      # Application entry point
│       └── main.go
├── internal/               # Internal packages (not exported)
│   ├── models/            # Domain models
│   │   └── bookmark.go
│   ├── repository/        # Data access layer
│   │   ├── repository.go  # Interfaces
│   │   └── sqlite.go      # SQLite implementation
│   ├── service/           # Business logic
│   │   └── service.go
│   ├── ui/                # User interface (TUI)
│   │   └── app.go
│   ├── parser/            # HTML bookmark parser
│   │   └── parser.go
│   ├── commands/          # CLI commands
│   │   └── import.go
│   └── config/            # Configuration
│       └── config.go
├── go.mod
└── README.md
```

## Architecture Layers

### 1. Models (Domain Models)
**Package:** `internal/models`

Contains basic data structures:
- `Bookmark` - bookmark structure
- `Folder` - folder structure

**Principles:**
- Data only, no logic
- Independent from other layers
- Can be used in all layers

### 2. Repository (Data Access Layer)
**Package:** `internal/repository`

**Interfaces:**
- `BookmarkRepository` - bookmark operations
- `FolderRepository` - folder operations
- `Repository` - combines all repositories

**Implementation:**
- `SQLiteRepository` - SQLite implementation

**Benefits:**
- Abstraction from specific database
- Easy to replace with another database (PostgreSQL, MySQL, etc.)
- Simplifies testing (can create mock implementation)

### 3. Service (Business Logic)
**Package:** `internal/service`

**Services:**
- `BookmarkService` - business logic for bookmarks
  - `ListAll()` - get all bookmarks
  - `Search(query)` - search bookmarks
  - `Create/Update/Delete` - CRUD operations
- `FolderService` - business logic for folders

**Principles:**
- Contains business logic (search, filtering)
- Does not depend on UI or database directly
- Works through repository interfaces

### 4. UI (User Interface)
**Package:** `internal/ui`

**Components:**
- `App` - main TUI application
- Uses `tview` for rendering
- Depends only on services, not repositories

**Principles:**
- Display and input handling only
- All logic delegated to services
- Easy to replace with another UI (web, GUI)

### 5. Parser
**Package:** `internal/parser`

**Functionality:**
- Parsing HTML bookmark files
- Uses services to create folders and bookmarks

### 6. Commands (CLI Commands)
**Package:** `internal/commands`

**Commands:**
- `ImportCommand` - import bookmarks from HTML

**Principles:**
- Each command is a separate type
- Easy to add new commands
- Isolated from UI

### 7. Config (Configuration)
**Package:** `internal/config`

**Functionality:**
- Application configuration management
- Default database path: `~/.bookmarks/bookmarks.db`
- Can be overridden via flags

## Data Flow

### Import Bookmarks
```
main.go → ImportCommand → Parser → FolderService → Repository → SQLite
                                    ↓
                              BookmarkService → Repository → SQLite
```

### Run TUI
```
main.go → UI.App → BookmarkService → Repository → SQLite
                → FolderService → Repository → SQLite
```

## Benefits of the New Architecture

### 1. Separation of Concerns
- Each package has a clear responsibility
- Easy to understand where everything is located

### 2. Testability
- Can easily create mock repositories for tests
- Services are tested independently from database
- UI can be tested with mock services

### 3. Extensibility
- Easy to add a new command (create a file in `commands/`)
- Easy to replace database (create a new `Repository` implementation)
- Easy to add a new UI (web interface, GUI)

### 4. Maintainability
- Changes in one layer do not affect others
- Easy to find and fix bugs
- Code is more readable and understandable

### 5. Dependency Injection
- Dependencies are passed through constructors
- No global variables
- Easy to manage dependencies

## Usage Examples

### Adding a New Command

```go
// internal/commands/export.go
type ExportCommand struct {
    repo repository.Repository
}

func (c *ExportCommand) Execute(outputPath string) error {
    // Export logic
}
```

### Adding a New Repository

```go
// internal/repository/postgres.go
type PostgresRepository struct {
    db *sql.DB
}

func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
    // Implementation
}
```

### Creating a Mock for Tests

```go
// internal/repository/mock.go
type MockRepository struct {
    bookmarks []models.Bookmark
}

func (m *MockRepository) Bookmarks() BookmarkRepository {
    return &mockBookmarkRepo{bookmarks: m.bookmarks}
}
```

## Migration from Old Architecture

Old files (`bookmark.go`, `db.go`, `ui.go`, `parser.go`, `main.go`) can be deleted after verifying the new architecture works correctly.

## Recommendations for Further Development

1. **Add tests:**
   - Unit tests for services
   - Integration tests for repositories
   - E2E tests for UI

2. **Add validation:**
   - URL validation in `BookmarkService`
   - Form data validation

3. **Improve error handling:**
   - Custom error types
   - Error logging

4. **Add configuration file:**
   - YAML/TOML config
   - Theme settings, hotkey configuration

5. **Add plugins:**
   - Export to various formats
   - Cloud service synchronization
