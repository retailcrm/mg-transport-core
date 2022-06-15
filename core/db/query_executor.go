package db

import "github.com/jinzhu/gorm"

// ExecStatements will execute a list of statements for the provided *gorm.DB.
// This method can be used to simplify migrations. Any other usage is discouraged.
func ExecStatements(db *gorm.DB, statements []string) error {
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
