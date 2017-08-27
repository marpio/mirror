package metadatastore

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/marpio/img-store/metadata"
)

type SqliteMetadataStore struct {
	db *sqlx.DB
}

func NewSqliteMetadataStore(dbName string) *SqliteMetadataStore {
	dbInstance := sqlx.MustConnect("sqlite3", dbName)
	initScript := `
	CREATE TABLE IF NOT EXISTS img (
		img_id text PRIMARY KEY,
		created_at DATETIME,
		img_hash text NOT NULL,
		b2_img_name text NOT NULL,
		b2_thumbnail_name text NOT NULL);`
	dbInstance.MustExec(initScript)
	return &SqliteMetadataStore{db: dbInstance}
}

func (datastore *SqliteMetadataStore) GetAll() ([]*metadata.Image, error) {
	var existingImgs = []*metadata.Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name FROM img;"); err != nil {
		log.Printf("Error quering images - err: %v", err)
		return nil, err
	}
	return existingImgs, nil
}
func (datastore *SqliteMetadataStore) GetByID(imgID string) ([]*metadata.Image, error) {
	var existingImgs = []*metadata.Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name FROM img WHERE img_id=$1 LIMIT 1;", imgID); err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *SqliteMetadataStore) Insert(imgEntity *metadata.Image) error {
	if _, err := datastore.db.NamedExec("INSERT INTO img (img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name) VALUES (:img_id, :created_at, :img_hash, :b2_img_name, :b2_thumbnail_name)", imgEntity); err != nil {
		log.Printf("Error inserting into DB: %v", err)
		return err
	}
	return nil
}

func (datastore *SqliteMetadataStore) DeleteIfImgChanged(imgID string, imgContentHash string) error {
	img, err := datastore.Exists(imgID)
	if err != nil {
		return err
	}
	imgExists := img != nil
	imgChanged := imgExists && img.ImgHash != imgContentHash
	if imgChanged {
		log.Print("Existing image changed.")
		err := datastore.Delete(imgID)
		if err != nil {
			log.Printf("Image with the ID = %v could not be deleted - err: %v", imgID, err)
			return err
		}
	}
	return nil
}

func (datastore *SqliteMetadataStore) Delete(imgID string) error {
	_, err := datastore.db.Exec("DELETE FROM img WHERE img_id=$1", imgID)
	if err != nil {
		return err
	}
	return nil
}

func (datastore *SqliteMetadataStore) Exists(imgID string) (*metadata.Image, error) {
	existingImgs, err := datastore.GetByID(imgID)
	if err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return nil, err
	}
	imgExists := len(existingImgs) > 0
	if imgExists {
		return existingImgs[0], nil
	}
	return nil, nil
}
