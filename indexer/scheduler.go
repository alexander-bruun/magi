package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/robfig/cron/v3"

	"github.com/alexander-bruun/magi/models"
)

var cacheDataDirectory = ""

// activeIndexers stores active indexer instances by library ID
var activeIndexers = make(map[uint]*Indexer)

// Indexer represents the indexer state (requred to be local type)
type Indexer struct {
	Library     models.Library
	Cron        *cron.Cron
	CronJobID   cron.EntryID
	CronRunning bool
	JobRunning  bool
	stop        chan struct{}
}

func Initialize(cacheDirectory string, libraries []models.Library) {
	log.Info("Initializing manga indexer!")
	cacheDataDirectory = cacheDirectory

	for _, library := range libraries {
		indexer := &Indexer{
			Library: library,
			stop:    make(chan struct{}),
		}

		go indexer.Start()
	}

	listener := &NotificationListener{
		notifications: make(chan models.Notification),
	}
	// Register indexer as a listener
	models.AddListener(listener)
}

// Start starts the Indexer for the given library
func (idx *Indexer) Start() {
	idx.Cron = cron.New()
	idx.CronJobID, _ = idx.Cron.AddFunc(idx.Library.Cron,
		idx.runIndexingJob)
	idx.Cron.Start()
	idx.CronRunning = true

	activeIndexers[idx.Library.ID] = idx

	log.Infof("Library indexer: '%s' has been registered (%s)!",
		idx.Library.Name,
		idx.Library.Cron)

	// Listen for stop signal
	for {
		select {
		case <-idx.stop:
			idx.Cron.Stop()
			idx.CronRunning = false
			log.Infof("Stop signal received for '%s' indexer. Cron scheduler stopped.",
				idx.Library.Name)
			return
		}
	}
}

// Stop stops the indexer
func (idx *Indexer) Stop() {
	if idx.CronRunning {
		idx.Cron.Stop()
		idx.CronRunning = false
		log.Infof("Stopping indexer for library: '%s'.",
			idx.Library.Name)
	}

	// Signal stop to the goroutine listening for updates
	close(idx.stop)

	// Remove from activeIndexers
	delete(activeIndexers,
		idx.Library.ID)
}

// runIndexingJob runs the indexing job
func (idx *Indexer) runIndexingJob() {
	if idx.JobRunning {
		log.Warnf("Skipping scheduled indexer job for library: '%s', a job is already running.",
			idx.Library.Name)
		return
	} else {
		idx.JobRunning = true
	}

	log.Infof("Indexing library: '%s'",
		idx.Library.Name)
	start := time.Now() // Record the start time of indexing

	defer func() {
		duration := time.Since(start)
		if idx.CronRunning {
			log.Infof("Indexer for library: '%s' completed in %s.",
				idx.Library.Name, duration)
		} else {
			log.Infof("Indexer for library: '%s' was stopped after %s.",
				idx.Library.Name, duration)
		}
	}()

	for _, folder := range idx.Library.Folders {
		// Check if the stop signal has been received
		select {
		case <-idx.stop:
			idx.JobRunning = false
			return // Stop processing this library's folders
		default:
			// Continue processing
		}
		log.Infof("Processing folder: %s", folder.Name)

		err := filepath.Walk(folder.Name, func(path string, info os.FileInfo, err error) error {
			// Check for stop signal
			select {
			case <-idx.stop:
				idx.JobRunning = false
				return filepath.SkipDir // Stop walking this directory
			default:
				// Continue processing
			}
			if err != nil {
				return err
			}
			if info.IsDir() {
				// Get the relative path to determine depth
				relPath, err := filepath.Rel(folder.Name, path)
				if err != nil {
					return err
				}
				// Split the relative path to count the number of components
				parts := strings.Split(relPath, string(os.PathSeparator))
				// If the directory is deeper than one level, skip it
				if len(parts) > 1 {
					return filepath.SkipDir
				}

				// Get the absolute path
				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}

				mangaID, err := IndexManga(absPath, idx.Library.ID)
				if err != nil {
					log.Errorf("Failed to index manga: %s", absPath)
					return filepath.SkipDir
				}

				err = IndexChapters(absPath, mangaID)
			}
			return nil
		})

		if err != nil {
			log.Errorf("Error walking the path: %v", err)
		}
	}
	idx.JobRunning = false
}

// NotificationListener is a global channel for all notifications
type NotificationListener struct {
	notifications chan models.Notification
}

func (nl *NotificationListener) Notify(notification models.Notification) {
	log.Infof("Notification of type %s named '%s' was received, taking action!",
		notification.Type,
		notification.Payload.(models.Library).Name)
	switch notification.Type {
	case "library_created":
		newLibrary := notification.Payload.(models.Library)
		indexer := &Indexer{
			Library: newLibrary,
			stop:    make(chan struct{}),
		}

		activeIndexers[newLibrary.ID] = indexer

		go indexer.Start()

	case "library_updated":
		updatedLibrary := notification.Payload.(models.Library)

		if existingIndexer, exists := activeIndexers[updatedLibrary.ID]; exists {
			// Stop the existing indexer
			existingIndexer.Stop()
			delete(activeIndexers, updatedLibrary.ID)
		}
		// Create and start a new indexer for the updated library
		newIndexer := &Indexer{
			Library: updatedLibrary,
			stop:    make(chan struct{}),
		}
		activeIndexers[updatedLibrary.ID] = newIndexer

		go newIndexer.Start()

	case "library_deleted":
		deletedLibraryID := notification.Payload.(models.Library).ID

		if existingIndexer, exists := activeIndexers[deletedLibraryID]; exists {
			existingIndexer.Stop()
			delete(activeIndexers, deletedLibraryID)
		}

	default:
		log.Warnf("Unknown notification type: %s", notification.Type)
	}
}
