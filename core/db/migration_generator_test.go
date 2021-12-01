package db

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type MigrationGeneratorSuite struct {
	suite.Suite
	command *NewMigrationCommand
}

func (s *MigrationGeneratorSuite) SetupSuite() {
	s.command = &NewMigrationCommand{Directory: "/tmp"}
}

func (s *MigrationGeneratorSuite) Test_FileExists() {
	var (
		seededRand = rand.New(rand.NewSource(time.Now().UnixNano())) // nolint:gosec
		notExist   = fmt.Sprintf("/tmp/%d", seededRand.Int31())
	)

	assert.False(s.T(), s.command.FileExists(notExist))
}

func (s *MigrationGeneratorSuite) Test_Execute() {
	found := false
	assert.NoError(s.T(), s.command.Execute([]string{}))
	files, err := ioutil.ReadDir(s.command.Directory)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if strings.Contains(f.Name(), "_app.go") {
			found = true
			assert.NoError(s.T(), os.Remove(path.Join(s.command.Directory, f.Name())))
		}
	}

	assert.True(s.T(), found)
}

func Test_MigrationGenerator(t *testing.T) {
	suite.Run(t, new(MigrationGeneratorSuite))
}
