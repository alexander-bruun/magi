package indexer

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/robfig/cron/v3"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/executor"
)

var (
	cacheDataDirectory = ""
	activeIndexers     = make(map[string]*Indexer)
	scannedPathCount   int
	scanMutex          sync.Mutex
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

	// Initialize scraper scheduler
	executor.InitializeScraperScheduler()

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
	idx.stopOnce.Do(func() {
		if idx.CronRunning {
			if idx.Cron != nil {
				idx.Cron.Remove(idx.CronJobID)
			}
			idx.Cron.Stop()
			idx.CronRunning = false
			log.Infof("Stopped indexer for library: '%s'", idx.Library.Name)
		}
		close(idx.stop)
		delete(activeIndexers, idx.Library.Slug)
	})
}

// runIndexingJob performs the indexing job
func (idx *Indexer) runIndexingJob() {
	if idx.JobRunning {
		log.Infof("Indexing job for library '%s' already running, skipping scheduled run", idx.Library.Name)
		return
	}
	defer func() {
		idx.JobRunning = false
		// Reset the scanned path count after indexing completes
		scanMutex.Lock()
		scannedPathCount = 0
		scanMutex.Unlock()
	}()

	idx.JobRunning = true
	log.Infof("Starting scheduled indexing for library '%s'", idx.Library.Name)
	start := time.Now()

	for _, folder := range idx.Library.Folders {
		if err := idx.processFolder(folder); err != nil {
			log.Errorf("Error processing folder '%s': %s", folder, err)
		}

		select {
		case <-idx.stop:
			log.Infof("Indexing for library '%s' interrupted", idx.Library.Name)
			return
		default:
		}
	}

	duration := time.Since(start)
	seconds := duration.Seconds()
	scanMutex.Lock()
	totalScanned := scannedPathCount
	scanMutex.Unlock()
	
	log.Infof(
		"Scheduled indexing for library '%s' completed in %.1fs (scanned %d manga paths)",
		idx.Library.Name,
		seconds,
		totalScanned,
	)

	go func(library models.Library) {
		mangas, err := models.GetMangasByLibrarySlug(library.Slug)
		if err != nil {
			log.Errorf("Failed to list mangas for cleanup for library '%s': %s", library.Name, err)
			return
		}

		for _, m := range mangas {
			if m.Path == "" {
				continue
			}
			
			// Check if the path no longer exists on disk
			if _, err := os.Stat(m.Path); os.IsNotExist(err) {
				log.Infof("Manga path missing on disk, deleting manga '%s' (slug=%s)", m.Name, m.Slug)
				if err := models.DeleteManga(m.Slug); err != nil {
					log.Errorf("Failed to delete manga '%s': %s", m.Slug, err)
				}
				continue
			}
			
			// Check if the path is still within one of the library's configured folders
			pathInLibrary := false
			for _, folder := range library.Folders {
				absFolder, err := filepath.Abs(folder)
				if err != nil {
					log.Warnf("Failed to get absolute path for folder '%s': %s", folder, err)
					continue
				}
				absMangaPath, err := filepath.Abs(m.Path)
				if err != nil {
					log.Warnf("Failed to get absolute path for manga '%s': %s", m.Path, err)
					continue
				}
				
				relPath, err := filepath.Rel(absFolder, absMangaPath)
				if err == nil && !strings.HasPrefix(relPath, "..") {
					pathInLibrary = true
					break
				}
			}
			
			if !pathInLibrary {
				log.Infof("Manga path '%s' no longer in library folders, deleting manga '%s' (slug=%s)", m.Path, m.Name, m.Slug)
				if err := models.DeleteManga(m.Slug); err != nil {
					log.Errorf("Failed to delete manga '%s': %s", m.Slug, err)
				}
			}
		}

		if err := cleanupOrphanedDuplicates(); err != nil {
			log.Errorf("Failed to cleanup orphaned duplicates for library '%s': %s", library.Name, err)
		}
	}(idx.Library)
}

// RunIndexingJob triggers the indexer job immediately
func (idx *Indexer) RunIndexingJob() {
	idx.runIndexingJob()
}

// ✅ UPDATED: processFolder now sorts folders alphabetically
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

	// ✅ Sort entries alphabetically by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		select {
		case <-idx.stop:
			return nil
		default:
		}

		path := filepath.Join(folder, entry.Name())
		if entry.IsDir() {
			// Increment the global scan counter
			scanMutex.Lock()
			scannedPathCount++
			currentCount := scannedPathCount
			scanMutex.Unlock()
			
			log.Debugf("Scanning manga path [%d]: %s", currentCount, path)
			
			if _, err := IndexManga(path, idx.Library.Slug); err != nil {
				log.Errorf("Error indexing manga at '%s': %s", path, err)
			}
		} else {
			// Check if file is an archive file (single-chapter manga)
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".cbz" || ext == ".cbr" || ext == ".zip" || ext == ".rar" {
				// Increment the global scan counter
				scanMutex.Lock()
				scannedPathCount++
				currentCount := scannedPathCount
				scanMutex.Unlock()
				
				log.Debugf("Scanning manga file [%d]: %s", currentCount, path)
				
				if _, err := IndexManga(path, idx.Library.Slug); err != nil {
					log.Errorf("Error indexing manga at '%s': %s", path, err)
				}
			} else {
				log.Debugf("Skipping non-manga file: %s", entry.Name())
			}
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

		if dup.FolderPath1 != "" {
			if _, err := os.Stat(dup.FolderPath1); os.IsNotExist(err) {
				folder1Exists = false
			}
		}

		if dup.FolderPath2 != "" {
			if _, err := os.Stat(dup.FolderPath2); os.IsNotExist(err) {
				folder2Exists = false
			}
		}

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
