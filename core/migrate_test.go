package core

import (
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
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
	var (
		db  *sql.DB
		err error
	)

	require.NotEmpty(m.T(), (MigrationInfo{}).TableName())

	db, m.mock, err = sqlmock.New()
	require.NoError(m.T(), err)

	m.DB, err = gorm.Open("postgres", db)
	require.NoError(m.T(), err)

	m.DB.LogMode(true)
	m.RefreshMigrate()
	m.Migrate.SetDB(m.DB)
}

func (m *MigrateTest) RefreshMigrate() {
	m.Migrate = &Migrate{
		db:         m.DB,
		prepared:   false,
		migrations: map[string]*gormigrate.Migration{},
	}
}

func (m *MigrateTest) Migration_TestModel() *gormigrate.Migration {
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

func (m *MigrateTest) Test_Add() {
	m.RefreshMigrate()
	m.Migrate.Add(nil)
	m.Migrate.Add(m.Migration_TestModel())

	assert.Equal(m.T(), 1, len(m.Migrate.migrations))
	i, ok := m.Migrate.migrations["1"]
	require.True(m.T(), ok)
	assert.Equal(m.T(), "1", i.ID)
}

func (m *MigrateTest) Test_prepareMigrations_NilDB() {
	m.RefreshMigrate()
	m.Migrate.db = nil
	err := m.Migrate.prepareMigrations()

	require.Error(m.T(), err)
	assert.Equal(m.T(), "db must not be nil", err.Error())
}

func (m *MigrateTest) Test_prepareMigrations_AlreadyPrepared() {
	m.RefreshMigrate()
	m.Migrate.prepared = true
	err := m.Migrate.prepareMigrations()

	require.NoError(m.T(), err)
	assert.Nil(m.T(), m.Migrate.GORMigrate)
}

func (m *MigrateTest) Test_prepareMigrations_OK() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModel())
	err := m.Migrate.prepareMigrations()

	require.NoError(m.T(), err)
	assert.True(m.T(), m.Migrate.prepared)
	assert.NotNil(m.T(), m.Migrate.GORMigrate)
}

func (m *MigrateTest) Test_Migrate() {
	m.RefreshMigrate()
	m.Migrate.Add(m.Migration_TestModel())

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
}

func TestMigrate_Migrate(t *testing.T) {
	assert.NotNil(t, Migrations())
}

func TestMigrate_Suite(t *testing.T) {
	suite.Run(t, new(MigrateTest))
}
