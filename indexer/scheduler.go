package indexer

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/robfig/cron/v3"

	"github.com/alexander-bruun/magi/models"
)

var (
	cacheDataDirectory = ""
	activeIndexers     = make(map[string]*Indexer)
)

// Indexer represents the state of an indexer
type Indexer struct {
	Library     models.Library
	Cron        *cron.Cron
	CronJobID   cron.EntryID
	CronRunning bool
	JobRunning  bool
	stop        chan struct{}
	stopOnce    sync.Once
}

// Initialize sets up indexers and notifications
func Initialize(cacheDirectory string, libraries []models.Library) {
	cacheDataDirectory = cacheDirectory
	log.Info("Initializing Indexer and Scheduler")

	for _, library := range libraries {
		indexer := NewIndexer(library)
		go indexer.Start()
	}

	// Register NotificationListener
	models.AddListener(&NotificationListener{
		notifications: make(chan models.Notification),
	})
}

// NewIndexer creates a new Indexer instance
func NewIndexer(library models.Library) *Indexer {
	return &Indexer{
		Library: library,
		stop:    make(chan struct{}),
	}
}

// Start initializes and starts the Indexer
func (idx *Indexer) Start() {
	idx.Cron = cron.New()
	var err error
	idx.CronJobID, err = idx.Cron.AddFunc(idx.Library.Cron, idx.runIndexingJob)
	if err != nil {
		log.Errorf("Error adding cron job: %s", err)
		return
	}
	idx.Cron.Start()
	idx.CronRunning = true

	activeIndexers[idx.Library.Slug] = idx

	log.Infof("Library indexer '%s' registered with cron schedule '%s'",
		idx.Library.Name, idx.Library.Cron)

	// Listen for stop signal
	<-idx.stop
	idx.Stop()
}

// Stop stops the indexer and cleans up
func (idx *Indexer) Stop() {
	// Ensure Stop is only executed once to avoid closing the stop channel twice
	idx.stopOnce.Do(func() {
		if idx.CronRunning {
			// Remove the cron job so it won't be scheduled again
			if idx.Cron != nil {
				idx.Cron.Remove(idx.CronJobID)
			}
			idx.Cron.Stop()
			idx.CronRunning = false
			log.Infof("Stopped indexer for library: '%s'", idx.Library.Name)
		}

		// close the stop channel to signal any goroutines waiting on it
		close(idx.stop)
		delete(activeIndexers, idx.Library.Slug)
	})
}

// runIndexingJob performs the indexing job
func (idx *Indexer) runIndexingJob() {
	if idx.JobRunning {
		log.Infof("Indexing job for library '%s' already running, skipping", idx.Library.Name)
		return
	}
	defer func() {
		idx.JobRunning = false
	}()

	idx.JobRunning = true
	log.Infof("Starting indexing for library '%s'", idx.Library.Name)
	start := time.Now()

	for _, folder := range idx.Library.Folders {
		if err := idx.processFolder(folder); err != nil {
			log.Errorf("Error processing folder '%s': %s", folder, err)
		}

		// Check for stop signal after each folder
		select {
		case <-idx.stop:
			log.Infof("Indexing for library '%s' interrupted", idx.Library.Name)
			return
		default:
		}
	}

	duration := time.Since(start)
	log.Infof("Indexing for library '%s' completed in %s", idx.Library.Name, duration)

	// Cleanup: remove any mangas from DB whose folders no longer exist on disk
	go func(library models.Library) {
		mangas, err := models.GetMangasByLibrarySlug(library.Slug)
		if err != nil {
			log.Errorf("Failed to list mangas for cleanup for library '%s': %s", library.Name, err)
			return
		}

		for _, m := range mangas {
			// If the path for the manga no longer exists on disk, delete the manga
			if m.Path == "" {
				continue
			}
			if _, err := os.Stat(m.Path); os.IsNotExist(err) {
				log.Infof("Manga path missing on disk, deleting manga '%s' (slug=%s)", m.Name, m.Slug)
				if err := models.DeleteManga(m.Slug); err != nil {
					log.Errorf("Failed to delete manga '%s': %s", m.Slug, err)
				}
			}
		}

		// Cleanup: remove duplicate entries where one or both folders no longer exist
		if err := cleanupOrphanedDuplicates(); err != nil {
			log.Errorf("Failed to cleanup orphaned duplicates for library '%s': %s", library.Name, err)
		}
	}(idx.Library)
}

// ProcessFolder processes files and directories in a given folder. This is a
// package-level function so it can be called from outside an Indexer. The
// librarySlug parameter is used when indexing discovered manga directories.
// If stop is non-nil it will be observed for cancellation and the function
// will return early when a value is received.
// RunIndexingJob triggers the indexer job immediately. It's exported so
// callers outside the package can request a manual scan while preserving
// the Indexer's logging and lifecycle behavior.
func (idx *Indexer) RunIndexingJob() {
	// reuse existing runIndexingJob implementation
	idx.runIndexingJob()
}

// processFolder processes files and directories in a given folder and uses
// the Indexer's internal state (like Library.Slug and stop channel). This
// function is unexported to keep the implementation private to the package.
func (idx *Indexer) processFolder(folder string) error {
	dir, err := os.Open(folder)
	if err != nil {
		return err
	}
	defer dir.Close()

	entries, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-idx.stop:
			return nil
		default:
		}

		path := filepath.Join(folder, entry.Name())
		if entry.IsDir() {
			if _, err := IndexManga(path, idx.Library.Slug); err != nil {
				log.Errorf("Error indexing manga at '%s': %s", path, err)
			}
		} else {
			log.Debugf("File: %s", entry.Name())
		}
	}
	return nil
}

// cleanupOrphanedDuplicates removes duplicate entries where one or both folders no longer exist on disk
func cleanupOrphanedDuplicates() error {
	duplicates, err := models.GetAllMangaDuplicates()
	if err != nil {
		return err
	}

	deletedCount := 0
	for _, dup := range duplicates {
		folder1Exists := true
		folder2Exists := true

		// Check if folder 1 exists
		if dup.FolderPath1 != "" {
			if _, err := os.Stat(dup.FolderPath1); os.IsNotExist(err) {
				folder1Exists = false
			}
		}

		// Check if folder 2 exists
		if dup.FolderPath2 != "" {
			if _, err := os.Stat(dup.FolderPath2); os.IsNotExist(err) {
				folder2Exists = false
			}
		}

		// If either folder no longer exists, delete the duplicate entry
		if !folder1Exists || !folder2Exists {
			log.Infof("Deleting orphaned duplicate entry for manga '%s' (ID=%d): folder1_exists=%v, folder2_exists=%v",
				dup.MangaSlug, dup.ID, folder1Exists, folder2Exists)
			
			if err := models.DeleteMangaDuplicateByID(dup.ID); err != nil {
				log.Errorf("Failed to delete orphaned duplicate %d: %v", dup.ID, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		log.Infof("Cleaned up %d orphaned duplicate entries", deletedCount)
	}

	return nil
}

// NotificationListener listens for notifications and handles them
type NotificationListener struct {
	notifications chan models.Notification
}

// Notify processes incoming notifications
func (nl *NotificationListener) Notify(notification models.Notification) {
	log.Debugf("Received notification of type '%s' for library '%s'", notification.Type, notification.Payload.(models.Library).Name)

	switch notification.Type {
	case "library_created":
		nl.handleLibraryCreated(notification.Payload.(models.Library))
	case "library_updated":
		nl.handleLibraryUpdated(notification.Payload.(models.Library))
	case "library_deleted":
		nl.handleLibraryDeleted(notification.Payload.(models.Library))
	default:
		log.Warnf("Unknown notification type: %s", notification.Type)
	}
}

func (nl *NotificationListener) handleLibraryCreated(newLibrary models.Library) {
	indexer := NewIndexer(newLibrary)
	activeIndexers[newLibrary.Slug] = indexer
	go indexer.Start()
}

func (nl *NotificationListener) handleLibraryUpdated(updatedLibrary models.Library) {
	if existingIndexer, exists := activeIndexers[updatedLibrary.Slug]; exists {
		existingIndexer.Stop()
		delete(activeIndexers, updatedLibrary.Slug)
	}

	newIndexer := NewIndexer(updatedLibrary)
	activeIndexers[updatedLibrary.Slug] = newIndexer
	go newIndexer.Start()
}

func (nl *NotificationListener) handleLibraryDeleted(deletedLibrary models.Library) {
	if existingIndexer, exists := activeIndexers[deletedLibrary.Slug]; exists {
		existingIndexer.Stop()
		delete(activeIndexers, deletedLibrary.Slug)
	}
}
