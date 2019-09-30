package core

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/gormigrate.v1"
)

type TestModel struct {
	Name string `gorm:"column:name; type:varchar(70)"`
}

func (TestModel) TableName() string {
	return "test_model"
}

type MigrateTest struct {
	suite.Suite
	DB      *gorm.DB
	Migrate *Migrate
	mock    sqlmock.Sqlmock
}

func (m *MigrateTest) SetupSuite() {
	require.NotEmpty(m.T(), (MigrationInfo{}).TableName())
	m.RefreshMigrate()
}

func (m *MigrateTest) RefreshMigrate() {
	var (
		db  *sql.DB
		err error
	)

	db, m.mock, err = sqlmock.New()
	require.NoError(m.T(), err)

	m.DB, err = gorm.Open("postgres", db)
	require.NoError(m.T(), err)

	m.DB.LogMode(true)
	m.Migrate = &Migrate{
		db:         m.DB,
		prepared:   false,
		migrations: map[string]*gormigrate.Migration{},
	}
}

func (m *MigrateTest) Migration_TestModelFirst() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "1",
		Migrate: func(db *gorm.DB) error {
			return db.AutoMigrate(TestModel{}).Error
		},
		Rollback: func(db *gorm.DB) error {
			return db.DropTable(TestModel{}).Error
		},
	}
}

func (m *MigrateTest) Migration_TestModelSecond() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "2",
		Migrate: func(db *gorm.DB) error {
			return db.Model(TestModel{}).ModifyColumn("name", "varchar(100)").Error
		},
		Rollback: func(db *gorm.DB) error {
			return db.Model(TestModel{}).ModifyColumn("name", "varchar(70)").Error
		},
	}
}

func (m *MigrateTest) Test_Add() {
	m.RefreshMigrate()
	m.Migrate.Add(nil)
	m.Migrate.Add(m.Migration_TestModelFirst())

	assert.Equal(m.T(), 1, len(m.Migrate.migrations))
	i, ok := m.Migrate.migrations["1"]
	require.True(m.T(), ok)
	assert.Equal(m.T(), "1", i.ID)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_prepareMigrations_NilDB() {
	m.RefreshMigrate()
	m.Migrate.db = nil
	err := m.Migrate.prepareMigrations()

	require.Error(m.T(), err)
	assert.Equal(m.T(), "db must not be nil", err.Error())
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_prepareMigrations_AlreadyPrepared() {
	m.RefreshMigrate()
	m.Migrate.prepared = true
	err := m.Migrate.prepareMigrations()

	require.NoError(m.T(), err)
	assert.Nil(m.T(), m.Migrate.GORMigrate)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_prepareMigrations_OK() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())
	err := m.Migrate.prepareMigrations()

	require.NoError(m.T(), err)
	assert.True(m.T(), m.Migrate.prepared)
	assert.NotNil(m.T(), m.Migrate.GORMigrate)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Migrate_Fail_NilDB() {
	m.RefreshMigrate()
	m.Migrate.SetDB(nil)
	m.Migrate.Add(m.Migration_TestModelFirst())

	err := m.Migrate.Migrate()

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Migrate_Success_NoMigrations() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())

	m.mock.ExpectBegin()
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT id FROM migrations`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.mock.ExpectCommit()

	err := m.Migrate.Migrate()

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Migrate_Success() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())

	m.mock.ExpectBegin()
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT id FROM migrations`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "test_model" ("name" varchar(70) )`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`INSERT INTO migrations (id) VALUES ($1)`)).
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectCommit()

	err := m.Migrate.Migrate()

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Rollback_Fail_NilDB() {
	m.RefreshMigrate()
	m.Migrate.SetDB(nil)
	m.Migrate.Add(m.Migration_TestModelFirst())

	err := m.Migrate.Rollback()

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Rollback_Fail_NoMigrations() {
	m.RefreshMigrate()
	m.Migrate.first = m.Migration_TestModelFirst()

	err := m.Migrate.Rollback()

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Rollback_Fail_NoFirstMigration() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())
	m.Migrate.first = nil

	err := m.Migrate.Rollback()

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_MigrateTo_Fail_NilDB() {
	m.RefreshMigrate()
	m.Migrate.SetDB(nil)

	err := m.Migrate.MigrateTo("version")

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_MigrateTo_DoNothing() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())

	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "migrations" ("id" varchar(255) , PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectBegin()
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT id FROM migrations`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.mock.ExpectCommit()

	err := m.Migrate.MigrateTo(m.Migration_TestModelFirst().ID)

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_MigrateTo() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())

	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "migrations" ("id" varchar(255) , PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectBegin()
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT id FROM migrations`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "test_model" ("name" varchar(70) )`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`INSERT INTO migrations (id) VALUES ($1)`)).
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectCommit()

	err := m.Migrate.MigrateTo(m.Migration_TestModelFirst().ID)

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_RollbackTo() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())
	m.Migrate.Add(m.Migration_TestModelSecond())

	m.mock.ExpectBegin()
	m.mock.ExpectCommit()

	err := m.Migrate.RollbackTo(m.Migration_TestModelSecond().ID)

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_MigrateNextTo() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())
	m.Migrate.Add(m.Migration_TestModelSecond())

	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "migrations" ("id" varchar(255) , PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectBegin()
	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE migrations (id VARCHAR(255) PRIMARY KEY)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT id FROM migrations`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.mock.
		ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "migrations"  WHERE (id = $1)`)).
		WithArgs("2").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`ALTER TABLE "test_model" ALTER COLUMN "name" TYPE varchar(100)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.
		ExpectExec(regexp.QuoteMeta(`INSERT INTO migrations (id) VALUES ($1)`)).
		WithArgs("2").
		WillReturnResult(sqlmock.NewResult(1, 1))
	m.mock.ExpectCommit()

	err := m.Migrate.MigrateNextTo(m.Migration_TestModelFirst().ID)

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_MigratePreviousTo() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModelFirst())
	m.Migrate.Add(m.Migration_TestModelSecond())

	m.mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "migrations" ("id" varchar(255) , PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := m.Migrate.MigratePreviousTo(m.Migration_TestModelSecond().ID)

	assert.Error(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func (m *MigrateTest) Test_Close() {
	m.RefreshMigrate()
	m.mock.ExpectClose()
	err := m.Migrate.Close()

	assert.NoError(m.T(), err)
	assert.NoError(m.T(), m.mock.ExpectationsWereMet())
}

func TestMigrate_Migrate(t *testing.T) {
	assert.NotNil(t, Migrations())
}

func TestMigrate_Suite(t *testing.T) {
	suite.Run(t, new(MigrateTest))
}
