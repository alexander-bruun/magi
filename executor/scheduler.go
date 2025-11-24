package executor

import (
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v2/log"
	cron "github.com/robfig/cron/v3"

	"github.com/alexander-bruun/magi/models"
)

var (
	scraperCron  *cron.Cron
	scraperMutex sync.Mutex
)

// InitializeScraperScheduler initializes the scraper scheduler and registers all enabled scripts
func InitializeScraperScheduler() {
	scraperMutex.Lock()
	defer scraperMutex.Unlock()

	if scraperCron != nil {
		scraperCron.Stop()
	}

	scraperCron = cron.New()
	scraperCron.Start()

	// Load and register all enabled scraper scripts
	scripts, err := models.ListScraperScripts(true)
	if err != nil {
		log.Errorf("Failed to load scraper scripts: %v", err)
		return
	}

	for _, script := range scripts {
		// create a local copy to avoid closing over the loop variable's address
		s := script
		if err := registerScraperScript(&s); err != nil {
			log.Errorf("Failed to register scraper script '%s': %v", script.Name, err)
		}
	}

	log.Debugf("Scraper scheduler initialized with %d scripts", len(scripts))
}

// registerScraperScript registers a single scraper script with the cron scheduler
func registerScraperScript(script *models.ScraperScript) error {
	if scraperCron == nil {
		return fmt.Errorf("scraper cron not initialized")
	}

	_, err := scraperCron.AddFunc(script.Schedule, func() {
		runScraperScript(script)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job for script '%s': %w", script.Name, err)
	}

	log.Infof("Script executor '%s' registered with cron schedule '%s'", script.Name, script.Schedule)
	return nil
}

// runScraperScript executes a scraper script
func runScraperScript(script *models.ScraperScript) {
	log.Infof("Starting scheduled script '%s' (ID=%d)", script.Name, script.ID)

	// Use shared executor for execution, broadcasting and cancel-tracking.
	if _, err := StartScriptExecution(script, script.Variables, true); err != nil {
		// If executor reports it's already running, that's expected — log and skip
		log.Infof("Scheduled run skipped: script '%s' (ID=%d) is already running: %v", script.Name, script.ID, err)
		return
	}
}

// ReloadScraperScheduler reloads the scraper scheduler (call after script changes)
func ReloadScraperScheduler() {
	log.Info("Reloading scraper scheduler")
	InitializeScraperScheduler()
}
