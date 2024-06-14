package db

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"text/template"
	"time"
)

var migrationTemplate = `package {{.Package}}

import (
	"github.com/jinzhu/gorm"
	"github.com/retailcrm/mg-transport-core/v2/core/db"
	"gopkg.in/gormigrate.v1"
)

func init() {
	db.Migrations().Add(&gormigrate.Migration{
		ID: "{{.Version}}",
		Migrate: func(db *gorm.DB) error {
			// Write your migration code here...
		},
		Rollback: func(db *gorm.DB) error {
			// Write your migration rollback code here...
		},
	})
}
`

// MigrationData contains base variables for the new migration.
type MigrationData struct {
	Package string
	Version string
}

// NewMigrationCommand struct.
type NewMigrationCommand struct {
	Directory string `short:"d" long:"directory" default:"./migrations" description:"Directory where migration will be created"` // nolint:lll
}

// FileExists returns true if provided file exist and it's not directory.
func (x *NewMigrationCommand) FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Execute migration generator command.
func (x *NewMigrationCommand) Execute(_ []string) error {
	tpl, err := template.New("migration").Parse(migrationTemplate)
	if err != nil {
		return fmt.Errorf("fatal: cannot parse base migration template: %w", err)
	}

	directory := path.Clean(x.Directory)
	migrationData := MigrationData{
		Package: "migrations",
		Version: strconv.FormatInt(time.Now().Unix(), 10),
	}

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return fmt.Errorf("err: specified directory doesn't exist")
	}

	if base := path.Base(directory); base != "/" && base != "." {
		migrationData.Package = base
	}

	filePath := path.Join(directory, migrationData.Version+"_app.go")
	if x.FileExists(filePath) {
		return fmt.Errorf("\"%s\" already exists or it's a directory", filePath)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	if err := tpl.Execute(file, migrationData); err != nil {
		return err
	}

	fmt.Println("Created new migration: " + filePath)
	return nil
}
