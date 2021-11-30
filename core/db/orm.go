package db

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/retailcrm/mg-transport-core/core/config"

	// PostgreSQL is an default.
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// ORM struct.
type ORM struct {
	DB *gorm.DB
}

// NewORM will init new database connection.
func NewORM(config config.DatabaseConfig) *ORM {
	orm := &ORM{}
	orm.CreateDB(config)
	return orm
}

// CreateDB connection using provided config.
func (orm *ORM) CreateDB(config config.DatabaseConfig) {
	db, err := gorm.Open("postgres", config.Connection)
	if err != nil {
		panic(err)
	}

	db.DB().SetConnMaxLifetime(time.Duration(config.ConnectionLifetime) * time.Second)
	db.DB().SetMaxOpenConns(config.MaxOpenConnections)
	db.DB().SetMaxIdleConns(config.MaxIdleConnections)

	db.SingularTable(true)
	db.LogMode(config.Logging)

	orm.DB = db
}

// CloseDB close database connection.
func (orm *ORM) CloseDB() {
	_ = orm.DB.Close()
}
