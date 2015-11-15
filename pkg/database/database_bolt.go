package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	//"github.com/enmand/quarid-go/pkg/logger"

	"github.com/boltdb/bolt"
)

func openBolt(constr string) (VMDatabase, error) {
	file, err := filepath.Abs(constr)
	_, err = os.Stat(file)
	if err != nil {
		return VMDatabase{nil}, fmt.Errorf("Unable to open '%s': %s", file, err)
	}
	d, err := bolt.Open(file, 0600, nil)

	return VMDatabase{d}, err
}

func NewBolt(name string) (*bolt.DB, error) {
	name += ".db"
	db, err := bolt.Open(name, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	return db, nil
}
