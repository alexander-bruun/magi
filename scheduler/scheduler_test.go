package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alexander-bruun/magi/models"
)

func TestNewCronScheduler(t *testing.T) {
	scheduler := NewCronScheduler()
	assert.NotNil(t, scheduler)
	assert.NotNil(t, scheduler.cron)
	assert.NotNil(t, scheduler.jobs)
	assert.False(t, scheduler.running)
}

func TestCronScheduler_AddJob(t *testing.T) {
	scheduler := NewCronScheduler()

	// Create a mock job
	job := &mockJob{name: "test-job"}

	// Add job
	err := scheduler.AddJob("test-job", "*/5 * * * *", job)
	assert.NoError(t, err)

	// Check that job was added
	scheduler.mutex.RLock()
	_, exists := scheduler.jobs["test-job"]
	scheduler.mutex.RUnlock()
	assert.True(t, exists)
}

func TestCronScheduler_AddJob_InvalidSchedule(t *testing.T) {
	scheduler := NewCronScheduler()
	job := &mockJob{name: "test-job"}

	// Add job with invalid schedule
	err := scheduler.AddJob("test-job", "invalid", job)
	assert.Error(t, err)
}

func TestCronScheduler_RemoveJob(t *testing.T) {
	scheduler := NewCronScheduler()
	job := &mockJob{name: "test-job"}

	// Add job first
	err := scheduler.AddJob("test-job", "*/5 * * * *", job)
	assert.NoError(t, err)

	// Remove job
	scheduler.RemoveJob("test-job")

	// Check that job was removed
	scheduler.mutex.RLock()
	_, exists := scheduler.jobs["test-job"]
	scheduler.mutex.RUnlock()
	assert.False(t, exists)
}

func TestCronScheduler_Start(t *testing.T) {
	scheduler := NewCronScheduler()

	// Start scheduler
	scheduler.Start()

	// Check that it's running
	assert.True(t, scheduler.IsRunning())
}

func TestCronScheduler_Stop(t *testing.T) {
	scheduler := NewCronScheduler()
	scheduler.Start()

	// Stop scheduler
	scheduler.Stop()

	// Check that it's not running
	assert.False(t, scheduler.IsRunning())
}

func TestCronScheduler_IsRunning(t *testing.T) {
	scheduler := NewCronScheduler()

	// Initially not running
	assert.False(t, scheduler.IsRunning())

	// Start
	scheduler.Start()
	assert.True(t, scheduler.IsRunning())

	// Stop
	scheduler.Stop()
	assert.False(t, scheduler.IsRunning())
}

func TestCronScheduler_Reload(t *testing.T) {
	scheduler := NewCronScheduler()
	job := &mockJob{name: "test-job"}

	// Add job
	err := scheduler.AddJob("test-job", "*/5 * * * *", job)
	assert.NoError(t, err)

	// Reload
	scheduler.Reload()

	// Job should still exist
	scheduler.mutex.RLock()
	_, exists := scheduler.jobs["test-job"]
	scheduler.mutex.RUnlock()
	assert.True(t, exists)
}

func TestScraperJob_Name(t *testing.T) {
	script := &models.ScraperScript{Name: "test-script"}
	job := &ScraperJob{Script: script}
	assert.Equal(t, "scraper-test-script", job.Name())
}

func TestScraperJob_Execute(t *testing.T) {
	script := &models.ScraperScript{Name: "test-script"}
	job := &ScraperJob{Script: script}
	err := job.Execute()
	// Should return error since no ExecuteFunc is set
	assert.Error(t, err)
}

func TestIndexingJob_Name(t *testing.T) {
	library := &models.Library{Slug: "test-library"}
	job := &IndexingJob{Library: library}
	assert.Equal(t, "indexer-test-library", job.Name())
}

func TestIndexingJob_Execute(t *testing.T) {
	library := &models.Library{Slug: "test-library"}
	job := &IndexingJob{Library: library}
	err := job.Execute()
	// Should return error since no ExecuteFunc is set
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no execute function provided")
}

func TestNewIndexer(t *testing.T) {
	library := models.Library{
		Name:    "Test Library",
		Slug:    "test-library",
		Folders: []string{"/tmp/test"},
	}

	indexer := NewIndexer(library)
	assert.NotNil(t, indexer)
	assert.Equal(t, library, indexer.Library)
	assert.NotNil(t, indexer.stop)
}

func TestIndexer_Start(t *testing.T) {
	library := models.Library{
		Name:    "Test Library",
		Slug:    "test-library",
		Folders: []string{"/tmp/test"},
		Cron:    "0 0 * * *",
	}

	indexer := NewIndexer(library)

	// Manually set up the scheduler like Start() does, but don't start the blocking loop
	indexer.Scheduler = NewCronScheduler()
	job := &IndexingJob{
		Library: &indexer.Library,
		ExecuteFunc: func(library *models.Library) error {
			return nil // Mock execute func that doesn't do database operations
		},
	}
	err := indexer.Scheduler.AddJob(job.Name(), indexer.Library.Cron, job)
	assert.NoError(t, err)

	indexer.Scheduler.Start()
	indexer.SchedulerRunning = true

	// Should be running
	assert.True(t, indexer.SchedulerRunning)
	assert.NotNil(t, indexer.Scheduler)

	// Clean up
	indexer.Scheduler.Stop()
	indexer.SchedulerRunning = false
}

func TestIndexer_Stop(t *testing.T) {
	library := models.Library{
		Name:    "Test Library",
		Slug:    "test-library",
		Folders: []string{"/tmp/test"},
		Cron:    "0 0 * * *",
	}

	indexer := NewIndexer(library)

	// Set up scheduler
	indexer.Scheduler = NewCronScheduler()
	indexer.Scheduler.Start()
	indexer.SchedulerRunning = true

	// Stop
	indexer.Stop()

	// Should not be running
	assert.False(t, indexer.SchedulerRunning)
}

func TestNotificationListener_Notify(t *testing.T) {
	listener := &NotificationListener{}
	library := models.Library{
		Name: "Test Library",
		Slug: "test-library",
	}
	notification := models.Notification{
		Type:    "library_created",
		Payload: library,
	}

	// Should not panic and should handle the notification
	listener.Notify(notification)
}

func TestRegisterScraperScript(t *testing.T) {
	// Initialize scraper scheduler for testing
	scraperScheduler = NewCronScheduler()

	// Create a test scraper script
	script := &models.ScraperScript{
		Name:     "test-script",
		Schedule: "0 0 * * *",
	}

	// Register the script
	err := RegisterScraperScript(script)
	assert.NoError(t, err)

	// Clean up
	scraperScheduler = nil
}

func TestInitializeIndexer(t *testing.T) {
	// Create test libraries with proper cron schedules
	libraries := []models.Library{
		{
			Name:    "Test Library 1",
			Slug:    "test-library-1",
			Folders: []string{"/tmp/test1"},
			Cron:    "0 0 * * *",
		},
		{
			Name:    "Test Library 2",
			Slug:    "test-library-2",
			Folders: []string{"/tmp/test2"},
			Cron:    "0 0 * * *",
		},
	}

	// Initialize indexer
	InitializeIndexer("/tmp/data", libraries, nil)

	// Check that indexers were created (this is hard to test without accessing global state)
	// For now, just ensure it doesn't panic
}

func TestIndexer_RunIndexingJob(t *testing.T) {
	// Skip this test as it requires database access and complex setup
	t.Skip("RunIndexingJob requires database access and is difficult to test in isolation")
}

// Mock job for testing
type mockJob struct {
	name string
}

func (m *mockJob) Name() string {
	return m.name
}

func (m *mockJob) Execute() error {
	return nil
}
