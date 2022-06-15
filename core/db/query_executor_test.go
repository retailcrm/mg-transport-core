package db

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecStatements(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open("postgres", sqlDB)
	require.NoError(t, err)

	mock.
		ExpectExec(regexp.QuoteMeta(`CREATE TABLE "test_users" ("username" varchar(255) NOT NULL, "id" serial, PRIMARY KEY ("id"))`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.
		ExpectExec(regexp.QuoteMeta(`INSERT INTO "test_users" ("username") VALUES ('username') RETURNING "test_users"."id"`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.
		ExpectExec(regexp.QuoteMeta(`DELETE FROM "test_users" WHERE (id = 1)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, ExecStatements(db, []string{
		`CREATE TABLE "test_users" ("username" varchar(255) NOT NULL, "id" serial, PRIMARY KEY ("id"))`,
		`INSERT INTO "test_users" ("username") VALUES ('username') RETURNING "test_users"."id"`,
		`DELETE FROM "test_users" WHERE (id = 1)`,
	}))
	assert.NoError(t, mock.ExpectationsWereMet())
}
