package indexer

import (
	"os"
	"path/filepath"
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
	if idx.CronRunning {
		idx.Cron.Stop()
		idx.CronRunning = false
		log.Infof("Stopped indexer for library: '%s'", idx.Library.Name)
	}

	close(idx.stop)
	delete(activeIndexers, idx.Library.Slug)
}

// runIndexingJob performs the indexing job
func (idx *Indexer) runIndexingJob() {
	if idx.JobRunning {
		log.Infof("Indexing job for library '%s' already running, skipping", idx.Library.Name)
		return
	}
	defer func() {
		idx.JobRunning = false
		log.Infof("Indexing job for library '%s' completed", idx.Library.Name)
	}()

	idx.JobRunning = true
	log.Infof("Starting indexing for library '%s'", idx.Library.Name)
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
	log.Infof("Indexing for library '%s' completed in %s", idx.Library.Name, duration)
}

// processFolder processes files and directories in a given folder
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
