package models

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestGetTotalMedias(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	count, err := GetTotalMedias()
	assert.NoError(t, err)
	assert.Equal(t, 42, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTotalChapters(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM chapters`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1234))

	count, err := GetTotalChapters()
	assert.NoError(t, err)
	assert.Equal(t, 1234, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTotalChaptersRead(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(567))

	count, err := GetTotalChaptersRead()
	assert.NoError(t, err)
	assert.Equal(t, 567, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTotalMediasByType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media WHERE LOWER\(TRIM\(type\)\) = LOWER\(TRIM\(\?\)\)`).
		WithArgs("manga").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

	count, err := GetTotalMediasByType("manga")
	assert.NoError(t, err)
	assert.Equal(t, 25, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTotalChaptersByType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM chapters c INNER JOIN media m ON c\.media_slug = m\.slug WHERE LOWER\(TRIM\(m\.type\)\) = LOWER\(TRIM\(\?\)\)`).
		WithArgs("novel").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(150))

	count, err := GetTotalChaptersByType("novel")
	assert.NoError(t, err)
	assert.Equal(t, 150, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChaptersReadCount(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE media_slug = \?`).
		WithArgs("manga1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	count, err := GetChaptersReadCount("manga1")
	assert.NoError(t, err)
	assert.Equal(t, 10, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTotalChaptersReadByType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states rs INNER JOIN media m ON rs\.media_slug = m\.slug WHERE LOWER\(TRIM\(m\.type\)\) = LOWER\(TRIM\(\?\)\)`).
		WithArgs("comic").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(75))

	count, err := GetTotalChaptersReadByType("comic")
	assert.NoError(t, err)
	assert.Equal(t, 75, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserTotalChaptersRead(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE user_name = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

	count, err := GetUserTotalChaptersRead("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 25, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserTotalMediaRead(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT COUNT\(DISTINCT media_slug\) FROM reading_states WHERE user_name = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))

	count, err := GetUserTotalMediaRead("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 12, count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserReadingStreak(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock the query for latest reading date
	mock.ExpectQuery(`SELECT DATE\(MAX\(created_at\)\) FROM reading_states WHERE user_name = \?`).
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"date"}).AddRow(time.Now()))

	// Mock the queries for counting consecutive days (today and yesterday)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE user_name = \? AND DATE\(created_at\) = DATE\(\?\)`).
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE user_name = \? AND DATE\(created_at\) = DATE\(\?\)`).
		WithArgs("testuser", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0)) // Break the streak

	streak, err := GetUserReadingStreak("testuser")
	assert.NoError(t, err)
	assert.Equal(t, 1, streak)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserFavoriteGenres(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"genres", "read_count"}).
		AddRow("Action, Adventure", 10).
		AddRow("Romance", 8).
		AddRow("Comedy", 5)

	mock.ExpectQuery(`SELECT m\.genres, COUNT\(\*\) as read_count FROM reading_states rs JOIN media m ON rs\.media_slug = m\.slug WHERE rs\.user_name = \? AND m\.genres != '' GROUP BY m\.genres ORDER BY read_count DESC LIMIT 5`).
		WithArgs("testuser").
		WillReturnRows(rows)

	genres, err := GetUserFavoriteGenres("testuser")
	assert.NoError(t, err)
	assert.Len(t, genres, 3)
	assert.Equal(t, "Action", genres[0])
	assert.Equal(t, "Romance", genres[1])
	assert.Equal(t, "Comedy", genres[2])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReadingActivityOverTime(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"date", "count"}).
		AddRow("2024-12-01", 5).
		AddRow("2024-12-02", 3).
		AddRow("2024-12-03", 8)

	mock.ExpectQuery(`SELECT DATE\(created_at\) as date, COUNT\(\*\) as count FROM reading_states WHERE created_at >= datetime\('now', '-' \|\| \? \|\| ' days'\) GROUP BY DATE\(created_at\) ORDER BY DATE\(created_at\)`).
		WithArgs(7).
		WillReturnRows(rows)

	activity, err := GetReadingActivityOverTime(7)
	assert.NoError(t, err)
	assert.Len(t, activity, 3)
	assert.Equal(t, 5, activity["2024-12-01"])
	assert.Equal(t, 3, activity["2024-12-02"])
	assert.Equal(t, 8, activity["2024-12-03"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopPopularSeriesByReads(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"name", "read_count"}).
		AddRow("Series A", 150).
		AddRow("Series B", 120).
		AddRow("Series C", 90)

	mock.ExpectQuery(`SELECT m\.name, COUNT\(rs\.id\) as read_count FROM media m INNER JOIN reading_states rs ON m\.slug = rs\.media_slug GROUP BY m\.slug, m\.name ORDER BY read_count DESC LIMIT \?`).
		WithArgs(5).
		WillReturnRows(rows)

	series, err := GetTopPopularSeriesByReads(5)
	assert.NoError(t, err)
	assert.Len(t, series, 3)
	assert.Equal(t, "Series A", series[0].Name)
	assert.Equal(t, 150, series[0].Count)
	assert.Equal(t, "Series B", series[1].Name)
	assert.Equal(t, 120, series[1].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopPopularSeriesByFavorites(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"name", "favorite_count"}).
		AddRow("Series A", 200).
		AddRow("Series B", 150).
		AddRow("Series C", 100)

	mock.ExpectQuery(`SELECT m\.name, COUNT\(f\.id\) as favorite_count FROM media m INNER JOIN favorites f ON m\.slug = f\.media_slug GROUP BY m\.slug, m\.name ORDER BY favorite_count DESC LIMIT \?`).
		WithArgs(5).
		WillReturnRows(rows)

	series, err := GetTopPopularSeriesByFavorites(5)
	assert.NoError(t, err)
	assert.Len(t, series, 3)
	assert.Equal(t, "Series A", series[0].Name)
	assert.Equal(t, 200, series[0].Count)
	assert.Equal(t, "Series B", series[1].Name)
	assert.Equal(t, 150, series[1].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopPopularSeriesByVotes(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"name", "vote_score"}).
		AddRow("Series A", 50).
		AddRow("Series B", 30).
		AddRow("Series C", 20)

	mock.ExpectQuery(`SELECT m\.name, COALESCE\(SUM\(CASE WHEN v\.value = 1 THEN 1 ELSE 0 END\), 0\) - COALESCE\(SUM\(CASE WHEN v\.value = -1 THEN 1 ELSE 0 END\), 0\) as vote_score FROM media m LEFT JOIN votes v ON m\.slug = v\.media_slug GROUP BY m\.slug, m\.name HAVING vote_score > 0 ORDER BY vote_score DESC LIMIT \?`).
		WithArgs(5).
		WillReturnRows(rows)

	series, err := GetTopPopularSeriesByVotes(5)
	assert.NoError(t, err)
	assert.Len(t, series, 3)
	assert.Equal(t, "Series A", series[0].Name)
	assert.Equal(t, 50, series[0].Count)
	assert.Equal(t, "Series B", series[1].Name)
	assert.Equal(t, 30, series[1].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCommentsActivityOverTime(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"date", "count"}).
		AddRow("2024-12-01", 8).
		AddRow("2024-12-02", 12).
		AddRow("2024-12-03", 15)

	mock.ExpectQuery(`SELECT DATE\(datetime\(created_at, 'unixepoch'\)\) as date, COUNT\(\*\) as count FROM comments WHERE created_at >= strftime\('%s', 'now', '-' \|\| \? \|\| ' days'\) GROUP BY DATE\(datetime\(created_at, 'unixepoch'\)\) ORDER BY DATE\(datetime\(created_at, 'unixepoch'\)\)`).
		WithArgs(7).
		WillReturnRows(rows)

	activity, err := GetCommentsActivityOverTime(7)
	assert.NoError(t, err)
	assert.Len(t, activity, 3)
	assert.Equal(t, 8, activity["2024-12-01"])
	assert.Equal(t, 12, activity["2024-12-02"])
	assert.Equal(t, 15, activity["2024-12-03"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetReviewsActivityOverTime(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	rows := sqlmock.NewRows([]string{"date", "count"}).
		AddRow("2024-12-01", 5).
		AddRow("2024-12-02", 8).
		AddRow("2024-12-03", 10)

	mock.ExpectQuery(`SELECT DATE\(datetime\(created_at, 'unixepoch'\)\) as date, COUNT\(\*\) as count FROM reviews WHERE created_at >= strftime\('%s', 'now', '-' \|\| \? \|\| ' days'\) GROUP BY DATE\(datetime\(created_at, 'unixepoch'\)\) ORDER BY DATE\(datetime\(created_at, 'unixepoch'\)\)`).
		WithArgs(7).
		WillReturnRows(rows)

	activity, err := GetReviewsActivityOverTime(7)
	assert.NoError(t, err)
	assert.Len(t, activity, 3)
	assert.Equal(t, 5, activity["2024-12-01"])
	assert.Equal(t, 8, activity["2024-12-02"])
	assert.Equal(t, 10, activity["2024-12-03"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopReadMedias(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Clear config cache to ensure config query is called
	configCacheTime = time.Time{}
	cachedConfig = AppConfig{}

	// Mock config query to fail (so it uses default content rating limit of 3)
	mock.ExpectQuery(`SELECT allow_registration.*FROM app_config WHERE id = 1`).
		WillReturnError(sqlmock.ErrCancelled)

	// Mock the main query for "week" period with limit 5
	// The query should filter by content ratings: safe, suggestive, erotica, pornographic (limit 3)
	expectedQuery := `SELECT m\.slug.*FROM media m.*LEFT JOIN.*SELECT media_slug.*COUNT.*as read_count.*FROM reading_states rs.*WHERE.*GROUP BY media_slug.*top_reads ON m\.slug.*LEFT JOIN.*SELECT media_slug.*CASE WHEN.*score FROM votes.*v ON v\.media_slug.*WHERE m\.content_rating IN.*AND m\.library_slug IN.*ORDER BY COALESCE.*DESC.*LIMIT`

	mock.ExpectQuery(expectedQuery).
		WithArgs("safe", "suggestive", "erotica", "pornographic", "lib1", 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"slug", "name", "author", "description", "year", "original_language", "type", "status", "content_rating", "library_slug", "cover_art_url", "path", "file_count", "read_count", "created_at", "updated_at",
		}).AddRow(
			"one-piece", "One Piece", "Eiichiro Oda", "A pirate adventure", 1997, "ja", "manga", "ongoing", "safe", "lib1", "/covers/one-piece.jpg", "/path/to/one-piece", 1000, 150, 1609459200, 1704067200,
		).AddRow(
			"naruto", "Naruto", "Masashi Kishimoto", "A ninja story", 1999, "ja", "manga", "completed", "safe", "lib1", "/covers/naruto.jpg", "/path/to/naruto", 700, 120, 1577836800, 1704067200,
		))

	media, err := GetTopReadMedias("week", 5, []string{"lib1"})
	assert.NoError(t, err)
	assert.Len(t, media, 2)
	assert.Equal(t, "one-piece", media[0].Slug)
	assert.Equal(t, "One Piece", media[0].Name)
	assert.Equal(t, 150, media[0].ReadCount)
	assert.Equal(t, "naruto", media[1].Slug)
	assert.Equal(t, "Naruto", media[1].Name)
	assert.Equal(t, 120, media[1].ReadCount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordDailyStatistics(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Mock GetTotalMedias
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM media`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	// Mock GetTotalChapters
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM chapters`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1234))

	// Mock GetTotalChaptersRead
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(567))

	// Mock the INSERT query
	mock.ExpectExec(`INSERT OR REPLACE INTO daily_statistics`).
		WithArgs(sqlmock.AnyArg(), 42, 1234, 567).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = RecordDailyStatistics()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDailyChange(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test for "media" stat type
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT media_slug\) FROM reading_states WHERE DATE\(created_at\) = \?`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	change, err := GetDailyChange("media")
	assert.NoError(t, err)
	assert.Equal(t, 5, change)

	// Test for "chapters" stat type
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reading_states WHERE DATE\(created_at\) = \?`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	change, err = GetDailyChange("chapters")
	assert.NoError(t, err)
	assert.Equal(t, 15, change)

	// Test for invalid stat type
	_, err = GetDailyChange("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stat type")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetDailyChangeByType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	// Test for "media" stat type
	mock.ExpectQuery(`SELECT.*COUNT\(\*\).*FROM media.*DATE\(created_at\).*LOWER\(TRIM\(type\)\)`).
		WithArgs(sqlmock.AnyArg(), "manga", sqlmock.AnyArg(), "manga").
		WillReturnRows(sqlmock.NewRows([]string{"change"}).AddRow(3))

	change, err := GetDailyChangeByType("media", "manga")
	assert.NoError(t, err)
	assert.Equal(t, 3, change)

	// Test for "chapters" stat type
	mock.ExpectQuery(`SELECT.*COUNT\(\*\).*FROM chapters.*INNER JOIN media.*DATE\(c\.created_at\)`).
		WithArgs(sqlmock.AnyArg(), "manga", sqlmock.AnyArg(), "manga").
		WillReturnRows(sqlmock.NewRows([]string{"change"}).AddRow(12))

	change, err = GetDailyChangeByType("chapters", "manga")
	assert.NoError(t, err)
	assert.Equal(t, 12, change)

	// Test for "chapters_read" stat type
	mock.ExpectQuery(`SELECT.*COUNT\(\*\).*FROM reading_states.*INNER JOIN media.*DATE\(rs\.created_at\)`).
		WithArgs(sqlmock.AnyArg(), "manga", sqlmock.AnyArg(), "manga").
		WillReturnRows(sqlmock.NewRows([]string{"change"}).AddRow(-2))

	change, err = GetDailyChangeByType("chapters_read", "manga")
	assert.NoError(t, err)
	assert.Equal(t, -2, change)

	// Test for invalid stat type
	_, err = GetDailyChangeByType("invalid", "manga")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown stat type")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopSeriesByComments(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT m\.name, COUNT\(c\.id\) as comment_count.*FROM media m.*INNER JOIN comments c ON m\.slug = c\.media_slug.*GROUP BY m\.slug, m\.name.*ORDER BY comment_count DESC.*LIMIT \?`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"name", "comment_count"}).
			AddRow("Series A", 25).
			AddRow("Series B", 18).
			AddRow("Series C", 12))

	result, err := GetTopSeriesByComments(10)
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "Series A", result[0].Name)
	assert.Equal(t, 25, result[0].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTopSeriesByReviews(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT m\.name, COUNT\(r\.id\) as review_count.*FROM media m.*INNER JOIN reviews r ON m\.slug = r\.media_slug.*GROUP BY m\.slug, m\.name.*ORDER BY review_count DESC.*LIMIT \?`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"name", "review_count"}).
			AddRow("Manga X", 15).
			AddRow("Manga Y", 8))

	result, err := GetTopSeriesByReviews(5)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Manga X", result[0].Name)
	assert.Equal(t, 15, result[0].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVoteDistribution(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT.*COALESCE\(SUM\(CASE WHEN value = 1 THEN 1 ELSE 0 END\), 0\) as upvotes.*COALESCE\(SUM\(CASE WHEN value = -1 THEN 1 ELSE 0 END\), 0\) as downvotes.*FROM votes`).
		WillReturnRows(sqlmock.NewRows([]string{"upvotes", "downvotes"}).AddRow(150, 25))

	upvotes, downvotes, err := GetVoteDistribution()
	assert.NoError(t, err)
	assert.Equal(t, 150, upvotes)
	assert.Equal(t, 25, downvotes)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMostControversialSeries(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT m\.name.*COALESCE\(SUM\(CASE WHEN v\.value = 1 THEN 1 ELSE 0 END\), 0\).*COALESCE\(SUM\(CASE WHEN v\.value = -1 THEN 1 ELSE 0 END\), 0\).*FROM media m.*LEFT JOIN votes v ON m\.slug = v\.media_slug.*GROUP BY m\.slug, m\.name.*HAVING.*ORDER BY.*LIMIT \?`).
		WithArgs(10).
		WillReturnRows(sqlmock.NewRows([]string{"name", "total_votes", "vote_diff"}).
			AddRow("Controversial Manga", 50, 5))

	result, err := GetMostControversialSeries(10)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Controversial Manga", result[0].Name)
	assert.Equal(t, 10, result[0].Count) // (5/50)*100 = 10

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetChaptersReadPerUserDistribution(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT.*CASE.*WHEN chapter_count = 0 THEN '0'.*WHEN chapter_count BETWEEN 1 AND 5 THEN '1-5'.*FROM.*SELECT user_name, COUNT\(\*\) as chapter_count.*FROM reading_states.*GROUP BY user_name.*GROUP BY range_bucket`).
		WillReturnRows(sqlmock.NewRows([]string{"range_bucket", "user_count"}).
			AddRow("1-5", 45).
			AddRow("6-10", 23).
			AddRow("11-20", 12).
			AddRow("21-50", 5))

	result, err := GetChaptersReadPerUserDistribution()
	assert.NoError(t, err)
	assert.Equal(t, 45, result["1-5"])
	assert.Equal(t, 23, result["6-10"])
	assert.Equal(t, 12, result["11-20"])
	assert.Equal(t, 5, result["21-50"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMostActiveReaders(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT user_name, COUNT\(\*\) as chapter_count.*FROM reading_states.*GROUP BY user_name.*ORDER BY chapter_count DESC.*LIMIT \?`).
		WithArgs(5).
		WillReturnRows(sqlmock.NewRows([]string{"user_name", "chapter_count"}).
			AddRow("reader1", 150).
			AddRow("reader2", 120))

	result, err := GetMostActiveReaders(5)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "reader1", result[0].Name)
	assert.Equal(t, 150, result[0].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAverageChaptersReadPerUser(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT AVG\(chapter_count\) FROM \( SELECT user_name, COUNT\(\*\) as chapter_count FROM reading_states GROUP BY user_name \)`).
		WillReturnRows(sqlmock.NewRows([]string{"avg"}).AddRow(42.5))

	result, err := GetAverageChaptersReadPerUser()
	assert.NoError(t, err)
	assert.Equal(t, 42.5, result)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserActivityByMediaType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT m\.type, COUNT\(\*\) as read_count.*FROM reading_states rs.*INNER JOIN media m ON rs\.media_slug = m\.slug.*WHERE m\.type IS NOT NULL.*GROUP BY m\.type`).
		WillReturnRows(sqlmock.NewRows([]string{"type", "read_count"}).
			AddRow("manga", 25).
			AddRow("manhwa", 15).
			AddRow("manhua", 10))

	result, err := GetUserActivityByMediaType()
	assert.NoError(t, err)
	assert.Equal(t, 25, result["manga"])
	assert.Equal(t, 15, result["manhwa"])
	assert.Equal(t, 10, result["manhua"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetNewMediaOverTime(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT DATE\(datetime\(created_at, 'unixepoch'\)\) as date, COUNT\(\*\) as count.*FROM media.*WHERE created_at >= strftime\('%s', 'now', '-' \|\| \? \|\| ' days'\).*GROUP BY DATE\(datetime\(created_at, 'unixepoch'\)\)`).
		WithArgs(30).
		WillReturnRows(sqlmock.NewRows([]string{"date", "count"}).
			AddRow("2024-01-01", 5).
			AddRow("2024-01-02", 3))

	result, err := GetNewMediaOverTime(30)
	assert.NoError(t, err)
	assert.Equal(t, 5, result["2024-01-01"])
	assert.Equal(t, 3, result["2024-01-02"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetNewChaptersOverTime(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT DATE\(datetime\(created_at, 'unixepoch'\)\) as date, COUNT\(\*\) as count.*FROM chapters.*WHERE created_at >= strftime\('%s', 'now', '-' \|\| \? \|\| ' days'\).*GROUP BY DATE\(datetime\(created_at, 'unixepoch'\)\)`).
		WithArgs(14).
		WillReturnRows(sqlmock.NewRows([]string{"date", "count"}).
			AddRow("2024-01-01", 12).
			AddRow("2024-01-02", 8))

	result, err := GetNewChaptersOverTime(14)
	assert.NoError(t, err)
	assert.Equal(t, 12, result["2024-01-01"])
	assert.Equal(t, 8, result["2024-01-02"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMediaGrowthByType(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	originalDB := db
	db = mockDB
	defer func() { db = originalDB }()

	mock.ExpectQuery(`SELECT type, COUNT\(\*\) as count FROM media WHERE type IS NOT NULL AND TRIM\(type\) != '' GROUP BY type ORDER BY count DESC`).
		WillReturnRows(sqlmock.NewRows([]string{"type", "count"}).
			AddRow("manga", 150).
			AddRow("manhwa", 75).
			AddRow("manhua", 25))

	result, err := GetMediaGrowthByType()
	assert.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "manga", result[0].Name)
	assert.Equal(t, 150, result[0].Count)

	assert.NoError(t, mock.ExpectationsWereMet())
}
