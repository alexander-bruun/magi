package handlers

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/alexander-bruun/magi/models"
	"github.com/alexander-bruun/magi/views"
	"github.com/gofiber/adaptor/v2"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	totalMedias = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_medias",
		Help: "Total number of media items",
	})

	totalChapters = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_chapters",
		Help: "Total number of chapters",
	})

	totalChaptersRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_chapters_read",
		Help: "Total number of chapters read",
	})

	totalUsers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_users",
		Help: "Total number of users",
	})

	totalLibraries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "magi_total_libraries",
		Help: "Total number of libraries",
	})

	imageLoadDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "magi_image_load_duration_seconds",
			Help:    "Time taken to serve images by file type",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"file_type"},
	)
)

// Cached metric values with TTL to prevent continuous database scans
var (
	cachedTotalMedias       float64
	cachedTotalChapters     float64
	cachedTotalChaptersRead float64
	cachedTotalUsers        float64
	cachedTotalLibraries    float64
	metricsCache            sync.Map // string -> *Metrics, concurrent
	lastMetricsUpdate       time.Time
	metricsCacheTTL         = 60 * time.Second // Cache metrics for 60 seconds
)

func init() {
	// Register metrics
	prometheus.MustRegister(totalMedias)
	prometheus.MustRegister(totalChapters)
	prometheus.MustRegister(totalChaptersRead)
	prometheus.MustRegister(totalUsers)
	prometheus.MustRegister(totalLibraries)
	prometheus.MustRegister(imageLoadDuration)
}

// updateMetrics updates all Prometheus metrics with current database values (cached)
func updateMetrics() {
	// No lock: lock-free update

	// Skip update if cache is still fresh
	if time.Since(lastMetricsUpdate) < metricsCacheTTL {
		totalMedias.Set(cachedTotalMedias)
		totalChapters.Set(cachedTotalChapters)
		totalChaptersRead.Set(cachedTotalChaptersRead)
		totalUsers.Set(cachedTotalUsers)
		totalLibraries.Set(cachedTotalLibraries)
		return
	}

	// Update media count
	if count, err := models.GetTotalMedias(); err == nil {
		cachedTotalMedias = float64(count)
		totalMedias.Set(cachedTotalMedias)
	} else {
		log.Warnf("Failed to get total medias for metrics: %v", err)
	}

	// Update chapter count
	if count, err := models.GetTotalChapters(); err == nil {
		cachedTotalChapters = float64(count)
		totalChapters.Set(cachedTotalChapters)
	} else {
		log.Warnf("Failed to get total chapters for metrics: %v", err)
	}

	// Update chapters read count
	if count, err := models.GetTotalChaptersRead(); err == nil {
		cachedTotalChaptersRead = float64(count)
		totalChaptersRead.Set(cachedTotalChaptersRead)
	} else {
		log.Warnf("Failed to get total chapters read for metrics: %v", err)
	}

	// Update user count
	if count, err := models.CountUsers(); err == nil {
		cachedTotalUsers = float64(count)
		totalUsers.Set(cachedTotalUsers)
	} else {
		log.Warnf("Failed to get total users for metrics: %v", err)
	}

	// Update library count
	if libraries, err := models.GetLibraries(); err == nil {
		cachedTotalLibraries = float64(len(libraries))
		totalLibraries.Set(cachedTotalLibraries)
	} else {
		log.Warnf("Failed to get total libraries for metrics: %v", err)
	}

	lastMetricsUpdate = time.Now()
}

// HandleMetrics serves Prometheus metrics
func HandleMetrics(c *fiber.Ctx) error {
	// Update metrics before serving
	updateMetrics()

	// Use Fiber adaptor to convert HTTP handler to Fiber handler
	return adaptor.HTTPHandler(promhttp.Handler())(c)
}

// HandleReady serves the readiness endpoint
func HandleReady(c *fiber.Ctx) error {
	// Check if database is accessible - basic connectivity check
	if err := models.PingDB(); err != nil {
		log.Errorf("Database not ready: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("NOT READY")
	}

	return c.SendString("OK")
}

// HandleHealth serves the health endpoint
func HandleHealth(c *fiber.Ctx) error {
	// Check database connectivity
	if err := models.PingDB(); err != nil {
		log.Errorf("Database health check failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("UNHEALTHY")
	}

	// Additional health check: try to perform a simple database query
	if _, err := models.GetTotalMedias(); err != nil {
		log.Errorf("Database query health check failed: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).SendString("UNHEALTHY")
	}

	return c.SendString("OK")
}

// HandleMetricsJSON serves metrics data in JSON format
func HandleMetricsJSON(c *fiber.Ctx) error {
	// Update metrics before serving
	updateMetrics()

	// Gather all metrics
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		log.Errorf("Failed to gather metrics: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to gather metrics")
	}

	// Find the image load duration metric
	imageLoadData := []map[string]interface{}{}
	for _, mf := range mfs {
		if mf.GetName() == "magi_image_load_duration_seconds" {
			for _, m := range mf.GetMetric() {
				data := map[string]interface{}{
					"count": m.GetHistogram().GetSampleCount(),
					"sum":   m.GetHistogram().GetSampleSum(),
				}
				for _, label := range m.GetLabel() {
					if label.GetName() == "file_type" {
						data["file_type"] = label.GetValue()
						break
					}
				}
				imageLoadData = append(imageLoadData, data)
			}
			break
		}
	}

	// Also get other metrics
	mediaCount := cachedTotalMedias
	chaptersCount := cachedTotalChapters
	chaptersReadCount := cachedTotalChaptersRead
	usersCount := cachedTotalUsers
	librariesCount := cachedTotalLibraries

	response := map[string]interface{}{
		"image_load_times": imageLoadData,
		"totals": map[string]interface{}{
			"medias":        mediaCount,
			"chapters":      chaptersCount,
			"chapters_read": chaptersReadCount,
			"users":         usersCount,
			"libraries":     librariesCount,
		},
	}

	return c.JSON(response)
}

// marshallIfOk marshals data and returns JSON string or empty object on error
func marshallIfOk(dataName string, data interface{}) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		log.Errorf("Failed to marshal %s: %v", dataName, err)
		return "{}"
	}
	return string(jsonBytes)
}

// seriesToMap converts a slice of SeriesData to a map
func seriesToMap(series []models.SeriesData) map[string]int {
	result := make(map[string]int)
	for _, data := range series {
		result[data.Name] = data.Count
	}
	return result
}

// groupUsersByCreationDate groups users by their creation date
func groupUsersByCreationDate(users []models.User) map[string]int {
	result := make(map[string]int)
	for _, user := range users {
		date := user.CreatedAt.Format("2006-01-02")
		result[date]++
	}
	return result
}

// HandleMonitoring renders the monitoring dashboard
func HandleMonitoring(c *fiber.Ctx) error {
	data := &models.MonitoringData{}

	// Get basic user data
	users, err := models.GetUsers()
	if err != nil {
		log.Errorf("Failed to get users for monitoring: %v", err)
		users = []models.User{}
	}
	data.UserData = marshallIfOk("user creation data", groupUsersByCreationDate(users))

	// Get tag usage statistics
	tagStats, err := models.GetTagUsageStats()
	if err != nil {
		log.Errorf("Failed to get tag usage stats: %v", err)
		tagStats = make(map[string]int)
	}
	data.TagData = marshallIfOk("tag stats", tagStats)

	// Get user role distribution
	roleStats, err := models.GetUserRoleDistribution()
	if err != nil {
		log.Errorf("Failed to get user role distribution: %v", err)
		roleStats = make(map[string]int)
	}
	data.RoleData = marshallIfOk("role stats", roleStats)

	// Get reading activity
	readingActivity, err := models.GetReadingActivityOverTime(30)
	if err != nil {
		log.Errorf("Failed to get reading activity: %v", err)
		readingActivity = make(map[string]int)
	}
	data.ReadingData = marshallIfOk("reading activity", readingActivity)

	// Get popular series data
	popularReadsSlice, err := models.GetTopPopularSeriesByReads(10)
	if err != nil {
		log.Errorf("Failed to get popular series by reads: %v", err)
		popularReadsSlice = []models.SeriesData{}
	}
	data.PopularReads = marshallIfOk("popular reads", seriesToMap(popularReadsSlice))

	popularFavoritesSlice, err := models.GetTopPopularSeriesByFavorites(10)
	if err != nil {
		log.Errorf("Failed to get popular series by favorites: %v", err)
		popularFavoritesSlice = []models.SeriesData{}
	}
	data.PopularFavorites = marshallIfOk("popular favorites", seriesToMap(popularFavoritesSlice))

	popularVotesSlice, err := models.GetTopPopularSeriesByVotes(10)
	if err != nil {
		log.Errorf("Failed to get popular series by votes: %v", err)
		popularVotesSlice = []models.SeriesData{}
	}
	data.PopularVotes = marshallIfOk("popular votes", seriesToMap(popularVotesSlice))

	// Get community engagement data
	commentsActivity, err := models.GetCommentsActivityOverTime(30)
	if err != nil {
		log.Errorf("Failed to get comments activity: %v", err)
		commentsActivity = make(map[string]int)
	}
	data.CommentsActivity = marshallIfOk("comments activity", commentsActivity)

	reviewsActivity, err := models.GetReviewsActivityOverTime(30)
	if err != nil {
		log.Errorf("Failed to get reviews activity: %v", err)
		reviewsActivity = make(map[string]int)
	}
	data.ReviewsActivity = marshallIfOk("reviews activity", reviewsActivity)

	topCommentedSlice, err := models.GetTopSeriesByComments(10)
	if err != nil {
		log.Errorf("Failed to get top commented series: %v", err)
		topCommentedSlice = []models.SeriesData{}
	}
	data.TopCommented = marshallIfOk("top commented", seriesToMap(topCommentedSlice))

	topReviewedSlice, err := models.GetTopSeriesByReviews(10)
	if err != nil {
		log.Errorf("Failed to get top reviewed series: %v", err)
		topReviewedSlice = []models.SeriesData{}
	}
	data.TopReviewed = marshallIfOk("top reviewed", seriesToMap(topReviewedSlice))

	// Get vote distribution data
	upvotes, downvotes, err := models.GetVoteDistribution()
	if err != nil {
		log.Errorf("Failed to get vote distribution: %v", err)
		upvotes, downvotes = 0, 0
	}
	voteDistribution := map[string]int{"upvotes": upvotes, "downvotes": downvotes}
	data.VoteDistribution = marshallIfOk("vote distribution", voteDistribution)

	controversialSlice, err := models.GetMostControversialSeries(10)
	if err != nil {
		log.Errorf("Failed to get most controversial series: %v", err)
		controversialSlice = []models.SeriesData{}
	}
	data.ControversialSeries = marshallIfOk("controversial series", seriesToMap(controversialSlice))

	// Get user activity levels data
	chaptersPerUserDistribution, err := models.GetChaptersReadPerUserDistribution()
	if err != nil {
		log.Errorf("Failed to get chapters per user distribution: %v", err)
		chaptersPerUserDistribution = make(map[string]int)
	}
	data.ChaptersDistribution = marshallIfOk("chapters distribution", chaptersPerUserDistribution)

	mostActiveReadersSlice, err := models.GetMostActiveReaders(10)
	if err != nil {
		log.Errorf("Failed to get most active readers: %v", err)
		mostActiveReadersSlice = []models.SeriesData{}
	}
	data.MostActiveReaders = marshallIfOk("most active readers", seriesToMap(mostActiveReadersSlice))

	activityByMediaType, err := models.GetUserActivityByMediaType()
	if err != nil {
		log.Errorf("Failed to get user activity by media type: %v", err)
		activityByMediaType = make(map[string]int)
	}
	data.ActivityByMediaType = marshallIfOk("activity by media type", activityByMediaType)

	// Get content growth data
	newMediaOverTime, err := models.GetNewMediaOverTime(30)
	if err != nil {
		log.Errorf("Failed to get new media over time: %v", err)
		newMediaOverTime = make(map[string]int)
	}
	data.NewMediaOverTime = marshallIfOk("new media over time", newMediaOverTime)

	newChaptersOverTime, err := models.GetNewChaptersOverTime(30)
	if err != nil {
		log.Errorf("Failed to get new chapters over time: %v", err)
		newChaptersOverTime = make(map[string]int)
	}
	data.NewChaptersOverTime = marshallIfOk("new chapters over time", newChaptersOverTime)

	mediaGrowthByTypeSlice, err := models.GetMediaGrowthByType()
	if err != nil {
		log.Errorf("Failed to get media growth by type: %v", err)
		mediaGrowthByTypeSlice = []models.SeriesData{}
	}
	data.MediaGrowthByType = marshallIfOk("media growth by type", seriesToMap(mediaGrowthByTypeSlice))

	// Get system stats
	systemStats, err := models.GetSystemStats()
	if err != nil {
		log.Errorf("Failed to get system stats: %v", err)
		systemStats = &models.SystemStats{}
	}
	data.SystemStats = marshallIfOk("system stats", systemStats)

	// Get disk stats
	diskStats, err := models.GetDiskStats()
	if err != nil {
		log.Errorf("Failed to get disk stats: %v", err)
		diskStats = []models.DiskStats{}
	}
	data.DiskStats = marshallIfOk("disk stats", diskStats)

	return HandleView(c, views.Monitoring(data))
}
