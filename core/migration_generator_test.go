package core

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

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

func (s *MigrationGeneratorSuite) Test_Execute() {
	found := false
	assert.NoError(s.T(), s.command.Execute([]string{}))
	files, err := ioutil.ReadDir(s.command.Directory)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if strings.Index(f.Name(), "_app.go") != -1 {
			found = true
			assert.NoError(s.T(), os.Remove(path.Join(s.command.Directory, f.Name())))
		}
	}

	assert.True(s.T(), found)
}
