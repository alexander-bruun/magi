package indexer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/log"
	cron "github.com/robfig/cron/v3"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/executor"
)

type ContentType int

const (
	MediaDirectory ContentType = iota
	LightnovelDirectory
	SingleMediaFile
	SingleLightNovelFile
	Skip
)

// Callback functions for job status notifications (set by handlers package)
var (
	NotifyIndexerStarted  func(librarySlug string, libraryName string)
	NotifyIndexerProgress func(librarySlug string, currentMedia string, progress string)
	NotifyIndexerFinished func(librarySlug string)
)

var (
	cacheDataDirectory = ""
	cacheDirMutex      sync.RWMutex
	activeIndexers     sync.Map
	scannedPathCount   int
	scanMutex          sync.Mutex
	indexingRunning   sync.Map
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
	// Deep copy the Folders slice and strings to prevent sharing underlying arrays and string backing
	foldersCopy := make([]string, len(library.Folders))
	for i, f := range library.Folders {
		foldersCopy[i] = string([]byte(f))
	}
	
	libraryCopy := library
	libraryCopy.Folders = foldersCopy
	libraryCopy.Slug = string([]byte(library.Slug))
	libraryCopy.Name = string([]byte(library.Name))
	libraryCopy.Description = string([]byte(library.Description))
	libraryCopy.Cron = string([]byte(library.Cron))
	
	return &Indexer{
		Library: libraryCopy,
		stop:    make(chan struct{}),
	}
}

// Start initializes and starts the Indexer
func (idx *Indexer) Start() {
	idx.Cron = cron.New()
	var err error
	idx.CronJobID, err = idx.Cron.AddFunc(idx.Library.Cron, func() { idx.runIndexingJob() })
	if err != nil {
		log.Errorf("Error adding cron job: %s", err)
		return
	}
	idx.Cron.Start()
	idx.CronRunning = true

	activeIndexers.Store(idx.Library.Slug, idx)

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
		activeIndexers.Delete(idx.Library.Slug)
		close(idx.stop)
	})
}

// runIndexingJob performs the indexing job
func (idx *Indexer) runIndexingJob() bool {
	if val, ok := indexingRunning.Load(idx.Library.Slug); ok && val.(bool) {
		log.Infof("Indexing job for library '%s' already running, skipping run", idx.Library.Name)
		return false
	}
	indexingRunning.Store(idx.Library.Slug, true)

	defer func() {
		indexingRunning.Delete(idx.Library.Slug)

		idx.JobRunning = false
		// Reset the scanned path count after indexing completes
		scanMutex.Lock()
		scannedPathCount = 0
		scanMutex.Unlock()

		// Notify that indexer has finished
		if NotifyIndexerFinished != nil {
			NotifyIndexerFinished(idx.Library.Slug)
		}
	}()

	idx.JobRunning = true

	// Notify that indexer has started
	if NotifyIndexerStarted != nil {
		NotifyIndexerStarted(idx.Library.Slug, idx.Library.Name)
	}

	// Get metadata provider name for logging
	providerName := "unknown"
	if config, err := models.GetAppConfig(); err == nil {
		if config.MetadataProvider != "" {
			providerName = config.MetadataProvider
		} else {
			providerName = "mangadex"
		}
	}

	log.Debugf("Starting scheduled indexing for library '%s' (metadata provider: %s)", idx.Library.Name, providerName)
	start := time.Now()

	for _, folder := range idx.Library.Folders {
		// Validate that the folder path looks reasonable
		if !filepath.IsAbs(folder) {
			log.Warnf("Library '%s' has relative folder path '%s', skipping", idx.Library.Name, folder)
			continue
		}
		
		absFolder, err := filepath.Abs(folder)
		if err != nil {
			log.Errorf("Failed to resolve folder path '%s' for library '%s': %s", folder, idx.Library.Name, err)
			continue
		}
		
		log.Debugf("Processing folder '%s' for library '%s'", absFolder, idx.Library.Name)
		if err := idx.processFolder(absFolder); err != nil {
			log.Errorf("Error processing folder '%s' for library '%s': %s", absFolder, idx.Library.Name, err)
		}

		select {
		case <-idx.stop:
			log.Infof("Indexing for library '%s' interrupted", idx.Library.Name)
			return true
		default:
		}
	}

	duration := time.Since(start)
	seconds := duration.Seconds()
	scanMutex.Lock()
	totalScanned := scannedPathCount
	scanMutex.Unlock()
	
	log.Debugf(
		"Scheduled indexing for library '%s' completed in %.1fs (scanned %d content paths)",
		idx.Library.Name,
		seconds,
		totalScanned,
	)

	go func(library models.Library) {
		mangas, err := models.GetMediasByLibrarySlug(library.Slug)
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
				log.Infof("Media path missing on disk, deleting media '%s' (slug=%s)", m.Name, m.Slug)
				if err := models.DeleteMedia(m.Slug); err != nil {
					log.Errorf("Failed to delete media '%s': %s", m.Slug, err)
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
				absMediaPath, err := filepath.Abs(m.Path)
				if err != nil {
					log.Warnf("Failed to get absolute path for media '%s': %s", m.Path, err)
					continue
				}
				
				relPath, err := filepath.Rel(absFolder, absMediaPath)
				if err == nil && !strings.HasPrefix(relPath, "..") {
					pathInLibrary = true
					break
				}
			}
			
			if !pathInLibrary {
				log.Infof("Media path '%s' no longer in library folders, deleting media '%s' (slug=%s)", m.Path, m.Name, m.Slug)
				if err := models.DeleteMedia(m.Slug); err != nil {
					log.Errorf("Failed to delete media '%s': %s", m.Slug, err)
				}
			}
		}

		if err := cleanupOrphanedDuplicates(); err != nil {
			log.Errorf("Failed to cleanup orphaned duplicates for library '%s': %s", library.Name, err)
		}
	}(idx.Library)

	return true
}

// RunIndexingJob triggers the indexer job immediately
func (idx *Indexer) RunIndexingJob() bool {
	return idx.runIndexingJob()
}

func (idx *Indexer) processFolder(folder string) error {
	// Validate that the folder path is reasonable for this library
	if !filepath.IsAbs(folder) {
		return fmt.Errorf("folder path must be absolute: %s", folder)
	}
	
	// Check if this folder is actually configured for this library
	folderConfigured := false
	for _, configuredFolder := range idx.Library.Folders {
		if absConfigured, err := filepath.Abs(configuredFolder); err == nil && absConfigured == folder {
			folderConfigured = true
			break
		}
	}
	
	if !folderConfigured {
		log.Warnf("Library '%s' trying to process folder '%s' which is not in its configured folders: %v", 
			idx.Library.Name, folder, idx.Library.Folders)
		return fmt.Errorf("folder not configured for this library: %s", folder)
	}
	
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

	// Collect media paths for concurrent processing
	var mediaPaths []string
	for _, entry := range entries {
		select {
		case <-idx.stop:
			return nil
		default:
		}

		path := filepath.Join(folder, entry.Name())
		contentType := determineContentType(path, entry.IsDir())

		switch contentType {
		case MediaDirectory, SingleMediaFile:
			mediaPaths = append(mediaPaths, path)
		default:
			log.Debugf("Skipping: %s", entry.Name())
		}
	}

	// Process media concurrently with worker pool
	const numWorkers = 4
	jobs := make(chan string, len(mediaPaths))
	results := make(chan error, len(mediaPaths))

	// Start workers
	for w := 0; w < numWorkers; w++ {
		go func() {
			for path := range jobs {
				select {
				case <-idx.stop:
					results <- nil
					return
				default:
				}

				// Increment the global scan counter
				scanMutex.Lock()
				scannedPathCount++
				currentCount := scannedPathCount
				scanMutex.Unlock()

				entryName := filepath.Base(path)
				log.Debugf("Scanning media [%d]: %s", currentCount, path)

				// Notify progress
				if NotifyIndexerProgress != nil {
					NotifyIndexerProgress(idx.Library.Slug, entryName, fmt.Sprintf("%d scanned", currentCount))
				}

				_, err := IndexMedia(path, idx.Library.Slug)
				results <- err
			}
		}()
	}

	// Send jobs
	for _, path := range mediaPaths {
		jobs <- path
	}
	close(jobs)

	// Collect results
	for i := 0; i < len(mediaPaths); i++ {
		if err := <-results; err != nil {
			path := mediaPaths[i]
			log.Errorf("Error indexing media at '%s': %s", path, err)
		}
	}

	return nil
}

// cleanupOrphanedDuplicates removes duplicate entries where one or both folders no longer exist on disk
func cleanupOrphanedDuplicates() error {
	duplicates, err := models.GetAllMediaDuplicates()
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
			log.Infof("Deleting orphaned duplicate entry for media '%s' (ID=%d): folder1_exists=%v, folder2_exists=%v",
				dup.MediaSlug, dup.ID, folder1Exists, folder2Exists)

			if err := models.DeleteMediaDuplicateByID(dup.ID); err != nil {
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
	activeIndexers.Store(newLibrary.Slug, indexer)
	go indexer.Start()
}

func (nl *NotificationListener) handleLibraryUpdated(updatedLibrary models.Library) {
	if val, ok := activeIndexers.Load(updatedLibrary.Slug); ok {
		existingIndexer := val.(*Indexer)
		existingIndexer.Stop()
		activeIndexers.Delete(updatedLibrary.Slug)
	}
	activeIndexers.Store(updatedLibrary.Slug, nil) // Placeholder while creating new indexer

	newIndexer := NewIndexer(updatedLibrary)
	activeIndexers.Store(updatedLibrary.Slug, newIndexer)
	go newIndexer.Start()
}

func (nl *NotificationListener) handleLibraryDeleted(deletedLibrary models.Library) {
	if val, ok := activeIndexers.Load(deletedLibrary.Slug); ok {
		existingIndexer := val.(*Indexer)
		existingIndexer.Stop()
		// Stop already removes from the map, so no need to delete again
		return
	}
}

// determineContentType determines if a path should be indexed as media
func determineContentType(path string, isDir bool) ContentType {
	if isDir {
		return MediaDirectory
	} else {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".cbz" || ext == ".cbr" || ext == ".zip" || ext == ".rar" {
			return SingleMediaFile
		}
	}
	return Skip
}

// containsEPUBFiles checks if a directory contains any .epub files
func containsEPUBFiles(dirPath string) bool {
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		return false
	}

	if !fileInfo.IsDir() {
		// For single files, check if it's an epub
		return strings.ToLower(filepath.Ext(dirPath)) == ".epub"
	}

	// For directories, walk and check for epub files
	var hasEPUB bool
	filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.ToLower(filepath.Ext(d.Name())) == ".epub" {
			hasEPUB = true
			return fs.SkipAll // Stop walking once we find one
		}
		return nil
	})

	return hasEPUB
}

// StopAllIndexers stops all running indexers
func StopAllIndexers() {
	activeIndexers.Range(func(key, value interface{}) bool {
		indexer := value.(*Indexer)
		if indexer != nil {
			log.Infof("Stopping indexer for library: %s", key.(string))
			close(indexer.stop)
		}
		activeIndexers.Delete(key)
		return true
	})
}
