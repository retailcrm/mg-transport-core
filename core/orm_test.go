package core

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestORM_NewORM(t *testing.T) {
	var (
		db  *sql.DB
		err error
	)

	defer func() {
		require.Nil(t, recover())
	}()

	db, _, err = sqlmock.New()
	require.NoError(t, err)

	config := DatabaseConfig{
		Connection:         db,
		Logging:            true,
		TablePrefix:        "",
		MaxOpenConnections: 10,
		MaxIdleConnections: 10,
		ConnectionLifetime: 100,
	}

	_ = NewORM(config)
}

func TestORM_createDB_Fail(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	NewORM(DatabaseConfig{Connection: nil})
}

func TestORM_CloseDB(t *testing.T) {
	var (
		db     *sql.DB
		dbMock sqlmock.Sqlmock
		err    error
	)

	defer func() {
		require.Nil(t, recover())
	}()

	db, dbMock, err = sqlmock.New()
	require.NoError(t, err)

	config := DatabaseConfig{
		Connection:         db,
		Logging:            true,
		TablePrefix:        "",
		MaxOpenConnections: 10,
		MaxIdleConnections: 10,
		ConnectionLifetime: 100,
	}

	dbMock.ExpectClose()
	orm := NewORM(config)
	orm.CloseDB()

	assert.NoError(t, dbMock.ExpectationsWereMet())
}
