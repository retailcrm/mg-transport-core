package testutil

import (
	"database/sql"
	"fmt"

	"github.com/jinzhu/gorm"
)

// DeleteCreatedEntities sets up GORM `onCreate` hook and return a function that can be deferred to
// remove all the entities created after the hook was set up.
// You can use it like this:
//
//      func TestSomething(t *testing.T){
//          db, _ := gorm.Open(...)
//          cleaner := DeleteCreatedEntities(db)
//          defer cleaner()
//      }.
func DeleteCreatedEntities(db *gorm.DB) func() { // nolint
	type entity struct { // nolint
		table   string
		keyname string
		key     interface{}
	}
	var entries []entity
	hookName := "cleanupHook"

	db.Callback().Create().After("gorm:create").Register(hookName, func(scope *gorm.Scope) {
		fmt.Printf("Inserted entities of %s with %s=%v\n", scope.TableName(), scope.PrimaryKey(), scope.PrimaryKeyValue())
		entries = append(entries, entity{table: scope.TableName(), keyname: scope.PrimaryKey(), key: scope.PrimaryKeyValue()})
	})
	return func() {
		// Remove the hook once we're done
		defer db.Callback().Create().Remove(hookName)
		// Find out if the current db object is already a transaction
		_, inTransaction := db.CommonDB().(*sql.Tx)
		tx := db
		if !inTransaction {
			tx = db.Begin()
		}
		// Loop from the end. It is important that we delete the entries in the
		// reverse order of their insertion
		for i := len(entries) - 1; i >= 0; i-- {
			entry := entries[i]
			fmt.Printf("Deleting entities from '%s' table with key %v\n", entry.table, entry.key)
			if err := tx.Table(entry.table).Where(entry.keyname+" = ?", entry.key).Delete("").Error; err != nil {
				panic(err)
			}
		}

		if !inTransaction {
			tx.Commit()
		}
	}
}
