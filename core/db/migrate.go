package db

import (
	"fmt"
	"sort"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"gopkg.in/gormigrate.v1"
)

// migrations default GORMigrate tool.
var migrations *Migrate

// Migrate tool, decorates gormigrate.Migration in order to provide better interface & versioning.
type Migrate struct {
	db         *gorm.DB
	first      *gormigrate.Migration
	migrations map[string]*gormigrate.Migration
	GORMigrate *gormigrate.Gormigrate
	versions   []string
	prepared   bool
}

// MigrationInfo with migration info.
type MigrationInfo struct {
	ID string `gorm:"column:id; type:varchar(255)"`
}

// TableName for MigrationInfo.
func (MigrationInfo) TableName() string {
	return "migrations"
}

// Migrations returns default migrate.
func Migrations() *Migrate {
	if migrations == nil {
		migrations = &Migrate{
			db:         nil,
			prepared:   false,
			migrations: map[string]*gormigrate.Migration{},
		}
	}

	return migrations
}

// Add GORMigrate to migrate.
func (m *Migrate) Add(migration *gormigrate.Migration) {
	if migration == nil {
		return
	}

	m.migrations[migration.ID] = migration
}

// SetDB to migrate.
func (m *Migrate) SetDB(db *gorm.DB) *Migrate {
	m.db = db
	return m
}

// Migrate all, including schema initialization.
func (m *Migrate) Migrate() error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	if len(m.migrations) > 0 {
		return m.GORMigrate.Migrate()
	}

	return nil
}

// Rollback all migrations.
func (m *Migrate) Rollback() error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	if m.first == nil {
		return errors.New("abnormal termination: first migration is nil")
	}

	if err := m.GORMigrate.RollbackTo(m.first.ID); err != nil {
		return err
	}

	if err := m.GORMigrate.RollbackMigration(m.first); err != nil {
		return err
	}

	return nil
}

// MigrateTo specified version.
func (m *Migrate) MigrateTo(version string) error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	current := m.Current()
	switch {
	case current > version:
		return m.GORMigrate.RollbackTo(version)
	case current < version:
		return m.GORMigrate.MigrateTo(version)
	default:
		return nil
	}
}

// MigrateNextTo migrate to next version from specified version.
func (m *Migrate) MigrateNextTo(version string) error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	if next, err := m.NextFrom(version); err == nil {
		current := m.Current()
		switch {
		case current < next:
			return m.GORMigrate.MigrateTo(next)
		case current > next:
			return fmt.Errorf("current migration version '%s' is higher than fetched version '%s'", current, next)
		default:
			return nil
		}
	} else {
		return nil
	}
}

// MigratePreviousTo migrate to previous version from specified version.
func (m *Migrate) MigratePreviousTo(version string) error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	if prev, err := m.PreviousFrom(version); err == nil {
		current := m.Current()
		switch {
		case current > prev:
			return m.GORMigrate.RollbackTo(prev)
		case current < prev:
			return fmt.Errorf("current migration version '%s' is lower than fetched version '%s'", current, prev)
		case prev == "0":
			return m.GORMigrate.RollbackMigration(m.first)
		default:
			return nil
		}
	} else {
		return nil
	}
}

// RollbackTo specified version.
func (m *Migrate) RollbackTo(version string) error {
	if err := m.prepareMigrations(); err != nil {
		return err
	}

	return m.GORMigrate.RollbackTo(version)
}

// Current migration version.
func (m *Migrate) Current() string {
	var migrationInfo MigrationInfo

	if m.db == nil {
		fmt.Println("warning => db is nil - cannot return migration version")
		return "0"
	}

	if !m.db.HasTable(MigrationInfo{}) {
		if err := m.db.CreateTable(MigrationInfo{}).Error; err == nil {
			fmt.Println("info => created migrations table")
		} else {
			panic(err.Error())
		}

		return "0"
	}

	if err := m.db.Last(&migrationInfo).Error; err != nil {
		fmt.Printf("warning => cannot fetch migration version: %s\n", err.Error())
		return "0"
	}

	return migrationInfo.ID
}

// NextFrom returns next version from passed version.
func (m *Migrate) NextFrom(version string) (string, error) {
	for key, ver := range m.versions {
		if ver == version {
			if key < (len(m.versions) - 1) {
				return m.versions[key+1], nil
			}

			return "", errors.New("this is last migration")
		}
	}

	return "", errors.New("cannot find specified migration")
}

// PreviousFrom returns previous version from passed version.
func (m *Migrate) PreviousFrom(version string) (string, error) {
	for key, ver := range m.versions {
		if ver == version {
			if key > 0 {
				return m.versions[key-1], nil
			}

			return "0", nil
		}
	}

	return "", errors.New("cannot find specified migration")
}

// Close db connection.
func (m *Migrate) Close() error {
	return m.db.Close()
}

// prepareMigrations prepare migrate.
func (m *Migrate) prepareMigrations() error {
	var (
		keys       []string
		migrations []*gormigrate.Migration
	)

	if m.db == nil {
		return errors.New("db must not be nil")
	}

	if m.prepared {
		return nil
	}

	i := 0
	keys = make([]string, len(m.migrations))
	for key := range m.migrations {
		keys[i] = key
		i++
	}

	sort.Strings(keys)
	m.versions = keys

	if len(keys) > 0 {
		if i, ok := m.migrations[keys[0]]; ok {
			m.first = i
		}
	}

	for _, key := range keys {
		if i, ok := m.migrations[key]; ok {
			migrations = append(migrations, i)
		}
	}

	options := &gormigrate.Options{
		TableName:                 gormigrate.DefaultOptions.TableName,
		IDColumnName:              gormigrate.DefaultOptions.IDColumnName,
		IDColumnSize:              gormigrate.DefaultOptions.IDColumnSize,
		UseTransaction:            true,
		ValidateUnknownMigrations: true,
	}

	m.GORMigrate = gormigrate.New(m.db, options, migrations)
	m.prepared = true
	return nil
}
