package core

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

var migrationTemplate = `package $package

import (
	"github.com/jinzhu/gorm"
	"github.com/retailcrm/mg-transport-core/core"
	"gopkg.in/gormigrate.v1"
)

func init() {
	core.Migrations().Add(&gormigrate.Migration{
		ID: "$version",
		Migrate: func(db *gorm.DB) error {
			// Write your migration code here...
		},
		Rollback: func(db *gorm.DB) error {
			// Write your migration rollback code here...
		},
	})
}
`

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
func (x *NewMigrationCommand) Execute(args []string) error {
	version := strconv.FormatInt(time.Now().Unix(), 10)
	directory := path.Clean(x.Directory)
	packageName := "migrations"

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return fmt.Errorf("err: specified directory doesn't exist")
	}

	if base := path.Base(directory); base != "/" && base != "." {
		packageName = base
	}

	filePath := path.Join(directory, version+"_app.go")
	if x.FileExists(filePath) {
		return fmt.Errorf("\"%s\" already exists or it's a directory", filePath)
	}

	migrationData := strings.Replace(
		strings.Replace(migrationTemplate, "$version", version, 1),
		"$package",
		packageName,
		1,
	)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	if _, err := file.WriteString(migrationData); err != nil {
		return err
	}

	fmt.Println("Created new migration: " + filePath)
	return nil
}
