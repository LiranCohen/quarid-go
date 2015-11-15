package database

import (
	//"fmt"
	//"strings"

	"github.com/boltdb/bolt"
	"upper.io/db"

	"github.com/enmand/quarid-go/pkg/config"
	//"github.com/enmand/quarid-go/pkg/crate"
	"github.com/enmand/quarid-go/pkg/logger"
)

var l = logger.Log

// Database is an interface for database access
type Database struct {
	db.Database
}

// VMDatabase is a database that can be used in a Crate's VM
type VMDatabase struct {
	*bolt.DB
}

// Get returns the configured, opened database, based on the current
// configuration
func Get() Database {
	c := config.Get()
	dbConfig := c.Get("database")
	uri, isString := dbConfig.(string)

	if !isString {
		if conurl, ok := dbConfig.(db.ConnectionURL); !ok {
			panic("Cannot connect to database")
		} else {
			uri = conurl.String()
		}
	}

	d, err := openPostgresql(uri)
	if err != nil {
		l.Errorf("Cannot open database %s: %s", uri, err)
		return Database{nil}
	}

	return d
}

// GetCrate returns a database for the given Crate
//func GetCrate(c *crate.Crate) VMDatabase {
//cfg := config.Get()
//dbConfig := cfg.GetString("vm.database")

//uri := strings.Split(dbConfig, "://") // uri[0] is proto, uri[1] is uri

//uri[1] = fmt.Sprintf("%s/%s", uri, c.ID)

//d, err := openBolt(uri[0])
//if err != nil {
//l.Errorf("Cannot open VM database %s", uri)
//return VMDatabase{nil}
//}

//return d
//}
