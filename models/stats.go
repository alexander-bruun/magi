package models

// Simple DB-backed counters for homepage statistics
func GetTotalMangas() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM mangas`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChapters() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM chapters`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}

func GetTotalChaptersRead() (int, error) {
    var count int
    row := db.QueryRow(`SELECT COUNT(*) FROM reading_states`)
    if err := row.Scan(&count); err != nil {
        return 0, err
    }
    return count, nil
}
