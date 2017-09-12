package sqlite

import (
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/marpio/img-store/metadatastore"
)

type SqliteMetadataStore struct {
	db *sqlx.DB
	sync.Mutex
}

func NewSqliteMetadataStore(dbName string) *SqliteMetadataStore {
	dbInstance := sqlx.MustConnect("sqlite3", dbName)
	initScript := `
	CREATE TABLE IF NOT EXISTS img (
		img_id text PRIMARY KEY,
		created_at DATETIME,
		created_at_month DATETIME,
		img_hash text NOT NULL,
		img_name text NOT NULL,
		thumbnail_name text NOT NULL,
		metadata_error INTEGER NOT NULL);`
	dbInstance.MustExec(initScript)
	return &SqliteMetadataStore{db: dbInstance}
}

func (datastore *SqliteMetadataStore) GetAll() ([]*metadatastore.Image, error) {
	var existingImgs = []*metadatastore.Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, created_at_month, img_hash, img_name, thumbnail_name FROM img WHERE metadata_error = 0;"); err != nil {
		log.Printf("Error quering images - err: %v", err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *SqliteMetadataStore) GetByMonth(month time.Time) ([]*metadatastore.Image, error) {
	var existingImgs = []*metadatastore.Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, created_at_month, img_hash, img_name, thumbnail_name FROM img WHERE created_at_month=$1 and metadata_error = 0;", month); err != nil {
		log.Printf("GetByMonth - Error quering images - err: %v", err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *SqliteMetadataStore) GetByID(imgID string) ([]*metadatastore.Image, error) {
	datastore.Lock()
	defer datastore.Unlock()
	var existingImgs = []*metadatastore.Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, created_at_month, img_hash, img_name, thumbnail_name, metadata_error FROM img WHERE img_id=$1 LIMIT 1;", imgID); err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *SqliteMetadataStore) Save(metadataEntities []*metadatastore.Image) (ok bool) {
	ok = true
	for _, imgEntity := range metadataEntities {
		if _, err := datastore.db.NamedExec("INSERT INTO img (img_id, created_at, created_at_month, img_hash, img_name, thumbnail_name, metadata_error) VALUES (:img_id, :created_at, :created_at_month, :img_hash, :img_name, :thumbnail_name, :metadata_error)", imgEntity); err != nil {
			log.Printf("Error inserting into DB: %v", err)
			ok = false
		}
	}
	return ok
}

func (datastore *SqliteMetadataStore) Delete(imgID string) error {
	_, err := datastore.db.Exec("DELETE FROM img WHERE img_id=$1", imgID)
	if err != nil {
		return err
	}
	return nil
}

func (datastore *SqliteMetadataStore) GetMonths() ([]*time.Time, error) {
	var res = []*time.Time{}
	if err := datastore.db.Select(&res, "SELECT DISTINCT created_at_month FROM img WHERE metadata_error = 0 ORDER BY created_at_month DESC;"); err != nil {
		log.Printf("Error getting created_at_month values - err: %v", err)
		return nil, err
	}
	return res, nil
}
