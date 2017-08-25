package metadatastore

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
)

type Image struct {
	ImgID           string    `db:"img_id"`
	CreatedAt       time.Time `db:"created_at"`
	ImgHash         string    `db:"img_hash"`
	B2ImgName       string    `db:"b2_img_name"`
	B2ThumbnailName string    `db:"b2_thumbnail_name"`
}

type FileStoreAction func() error
type Datastore interface {
	GetByID(imgID string) ([]Image, error)
	Insert(imgEntity *Image, actionFn FileStoreAction) error
	Update(imgID string, imgContentHash string) error
}

type db struct {
	db *sqlx.DB
}

func NewMetadataStore(dbName string, initScript string) Datastore {
	dbInstance := sqlx.MustConnect("sqlite3", dbName)
	dbInstance.MustExec(initScript)
	return &db{db: dbInstance}
}

func (datastore *db) GetByID(imgID string) ([]Image, error) {
	var existingImgs = []Image{}
	if err := datastore.db.Select(&existingImgs, "SELECT img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name FROM img WHERE img_id=$1 LIMIT 1;", imgID); err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return nil, err
	}
	return existingImgs, nil
}

func (datastore *db) Insert(imgEntity *Image, actionFn FileStoreAction) error {
	tx := datastore.db.MustBegin()
	if _, err := tx.NamedExec("INSERT INTO img (img_id, created_at, img_hash, b2_img_name, b2_thumbnail_name) VALUES (:img_id, :created_at, :img_hash, :b2_img_name, :b2_thumbnail_name)", imgEntity); err != nil {
		log.Printf("Error inserting into DB: %v", err)
		return err
	}

	if err := actionFn(); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil

}

func (datastore *db) Update(imgID string, imgContentHash string) error {
	existingImgs, err := datastore.GetByID(imgID)
	if err != nil {
		log.Printf("Error quering existing image %v - err: %v", imgID, err)
		return err
	}
	imgExists := len(existingImgs) > 0
	existingImgChanged := imgExists && existingImgs[0].ImgHash != imgContentHash
	if imgExists && !existingImgChanged {
		log.Print("Existing image unchanged - do nothing and process next img")
		return nil
	} else if existingImgChanged {
		log.Print("Existing image changed.")
		_, err := datastore.db.Exec("DELETE FROM img WHERE img_id=$1", imgID)
		if err != nil {
			log.Printf("Image with the ID = %v could not be deleted - err: %v", imgID, err)
			return err
		}
	}
	return nil
}
