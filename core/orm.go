package core

import (
	"time"

	"github.com/jinzhu/gorm"
	// PostgreSQL is an default
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

// ORM struct
type ORM struct {
	DB *gorm.DB
}

// NewORM will init new database connection
func NewORM(config DatabaseConfig) *ORM {
	orm := &ORM{}
	orm.createDB(config)
	return orm
}

func (orm *ORM) createDB(config DatabaseConfig) {
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

// CloseDB close database connection
func (orm *ORM) CloseDB() {
	_ = orm.DB.Close()
}
