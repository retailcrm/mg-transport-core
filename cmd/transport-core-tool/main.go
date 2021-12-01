package main

import (
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/retailcrm/mg-transport-core/v2/core/db"
)

// Options for tool command.
type Options struct{}

var (
	options = Options{}
	parser  = flags.NewParser(&options, flags.Default)
)

func init() {
	_, err := parser.AddCommand("migration",
		"Create new empty migration in specified directory.",
		"Create new empty migration in specified directory.",
		&db.NewMigrationCommand{},
	)

	if err != nil {
		panic(err.Error())
	}
}

func main() {
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp { // nolint:errorlint
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
}
