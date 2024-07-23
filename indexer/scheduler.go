package indexer

import (
	"os"
	"path/filepath"
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
	log.Info("Initializing Indexer and Scheduler")
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
		log.Infof("Skipping scheduled indexer job for library: '%s', a job is already running.", idx.Library.Name)
		return
	} else {
		idx.JobRunning = true
	}

	log.Infof("Indexing library: '%s'", idx.Library.Name)
	start := time.Now() // Record the start time of indexing

	defer func() {
		duration := time.Since(start)
		if idx.CronRunning {
			log.Infof("Indexer for library: '%s' completed in %s.", idx.Library.Name, duration)
		} else {
			log.Infof("Indexer for library: '%s' was stopped after %s.", idx.Library.Name, duration)
		}
	}()

outerLoop:
	for _, folder := range idx.Library.Folders {
		select {
		case <-idx.stop:
			idx.JobRunning = false
			break outerLoop
		default:
		}

		// Open the directory
		dir, err := os.Open(folder)
		if err != nil {
			log.Errorf("Error opening directory: '%s' (%s)", folder, err)
			continue
		}
		defer dir.Close()

		// Read the directory entries
		entries, err := dir.Readdir(-1)
		if err != nil {
			log.Errorf("Error reading directory: '%s' (%s)", folder, err)
			continue
		}

		for _, entry := range entries {
			select {
			case <-idx.stop:
				idx.JobRunning = false
				break outerLoop
			default:
			}
			path := filepath.Join(folder, entry.Name())
			if entry.IsDir() {
				_, err = IndexManga(path, idx.Library.ID)
				if err != nil {
					continue
				}
			} else {
				log.Debugf("File:", entry.Name())
			}
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
