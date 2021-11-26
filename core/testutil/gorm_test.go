package testutil

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testUser struct {
	ID       int    `gorm:"primary_key"`
	Username string `gorm:"column:username; type:varchar(255); not null;"`
}

func TestDeleteCreatedEntities(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open("postgres", sqlDB)
	require.NoError(t, err)

	mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "test_users" ("id" serial,"username" varchar(255) NOT NULL , PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectBegin()
	mock.
		ExpectQuery(regexp.QuoteMeta(`INSERT INTO "test_users" ("username") VALUES ($1) RETURNING "test_users"."id"`)).
		WithArgs("user").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("1"))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.
		ExpectExec(regexp.QuoteMeta(`DELETE FROM "test_users" WHERE (id = $1)`)).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	cleaner := DeleteCreatedEntities(db)

	require.NoError(t, db.AutoMigrate(&testUser{}).Error)
	require.NoError(t, db.Create(&testUser{Username: "user"}).Error)

	cleaner()

	assert.NoError(t, mock.ExpectationsWereMet())
}
