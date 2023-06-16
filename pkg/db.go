package pkg

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "image_data.db")
	if err != nil {
		return nil, err
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS image_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		page_url TEXT NOT NULL,
		image_url TEXT NOT NULL,
		original_size INTEGER,
		optimized_size INTEGER
	);
`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func insertImageData(db *sql.DB, pageURL string, result imageSizeResult) (int64, error) {
	insertQuery := `
		INSERT INTO image_data (page_url, image_url, original_size, optimized_size)
		VALUES (?, ?, ?, ?);
	`

	res, err := db.Exec(insertQuery, pageURL, result.imageURL, result.originalSize, result.optimizedSize)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func getImageDataByURL(db *sql.DB, pageURL string, imageURL string) (*imageSizeResult, error) {
	query := `
		SELECT original_size, optimized_size
		FROM image_data
		WHERE page_url = ? AND image_url = ?;
	`

	row := db.QueryRow(query, pageURL, imageURL)

	var result imageSizeResult
	err := row.Scan(&result.originalSize, &result.optimizedSize)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	result.imageURL = imageURL
	return &result, nil
}
