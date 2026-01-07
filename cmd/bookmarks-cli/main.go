package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/dastanaron/bookmarks/internal/commands"
	"github.com/dastanaron/bookmarks/internal/config"
	"github.com/dastanaron/bookmarks/internal/repository"
	"github.com/dastanaron/bookmarks/internal/service"
	"github.com/dastanaron/bookmarks/internal/ui"
)

func main() {
	importPath := flag.String("import", "", "Path to HTML bookmarks file to import")
	exportPath := flag.String("export", "", "Path to HTML bookmarks file to export")
	clearDoubles := flag.Bool("clear-doubles", false, "Remove duplicate bookmarks (same URL)")
	dbPath := flag.String("db", "", "Path to database file (default: ~/.bookmarks/bookmarks.db)")
	flag.Parse()

	cfg := config.NewConfig()
	if *dbPath != "" {
		cfg.WithDBPath(*dbPath)
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	repo, err := repository.NewSQLiteRepository(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()

	// Handle import command
	if *importPath != "" {
		importCmd := commands.NewImportCommand(repo)
		if err := importCmd.Execute(*importPath); err != nil {
			log.Fatalf("Import failed: %v", err)
		}
		return
	}

	// Handle export command
	if *exportPath != "" {
		exportCmd := commands.NewExportCommand(repo)
		if err := exportCmd.Execute(*exportPath); err != nil {
			log.Fatalf("Export failed: %v", err)
		}
		return
	}

	// Handle clear doubles command
	if *clearDoubles {
		clearCmd := commands.NewClearDoublesCommand(repo)
		if err := clearCmd.Execute(); err != nil {
			log.Fatalf("Clear doubles failed: %v", err)
		}
		return
	}

	// Run TUI application
	bookmarkSvc := service.NewBookmarkService(repo)
	folderSvc := service.NewFolderService(repo)
	app := ui.NewApp(bookmarkSvc, folderSvc)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
