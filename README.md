## Message Gateway Transport Library
[![Build Status](https://github.com/retailcrm/mg-transport-core/workflows/ci/badge.svg)](https://github.com/retailcrm/mg-transport-core/actions?query=workflow%3Aci)
[![Coverage](https://codecov.io/gh/retailcrm/mg-transport-core/branch/master/graph/badge.svg?logo=codecov&logoColor=white)](https://codecov.io/gh/retailcrm/mg-transport-core)
[![GitHub release](https://img.shields.io/github/release/retailcrm/mg-transport-core.svg?logo=github&logoColor=white)](https://github.com/retailcrm/mg-transport-core/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/retailcrm/mg-transport-core)](https://goreportcard.com/report/github.com/retailcrm/mg-transport-core)
[![GoLang version](https://img.shields.io/badge/go->=1.12-blue.svg?logo=go&logoColor=white)](https://golang.org/dl/)
[![pkg.go.dev](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/core)

This library provides different functions like error-reporting, logging, localization, etc. in order to make it easier to create transports.

Usage:
```go
package main

import (
    "os"
    "fmt"
    "html/template"

    "github.com/gin-gonic/gin"
    "github.com/retailcrm/mg-transport-core/core"
)

func main() {
    // Create new core.Engine instance
    app := core.New()

    // Load configuration
    app.Config = core.NewConfig("config.yml")

    // Set default error translation key (will be returned if something goes wrong)
    app.DefaultError = "unknown_error"

    // Set translations path
    app.TranslationsPath = "./translations"

    // Preload some translations so they will not be loaded for every request
    app.PreloadLanguages = core.DefaultLanguages
    
    // Configure gin.Engine inside core.Engine
    app.ConfigureRouter(func(engine *gin.Engine) {
        engine.Static("/static", "./static")
        engine.HTMLRender = app.CreateRenderer(
]           // Insert templates here. Custom functions also can be provided.
            // Default transl function will be injected automatically
            func(renderer *core.Renderer) {
                // Push method will load template from FS or from binary
                r.Push("home", "templates/layout.html", "templates/home.html")
            }, 
            template.FuncMap{},
        )
    })
    
    // Start application or fail if something gone wrong (e.g. port is already in use)
    if err := app.Prepare().Run(); err != nil {
        fmt.Printf("Fatal error: %s", err.Error())
        os.Exit(1)
    }
}
```

### Resource embedding
[embed](https://golang.org/pkg/embed/) can be used to provide resource embedding. Go source files that import "embed" can use the //go:embed directive to initialize a variable of type string, []byte, or FS with the contents of files read from the package directory or subdirectories at compile time.

Example:
```go
package main

import (
    "os"
    "fmt"
    "html/template"
    "io/fs"
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/retailcrm/mg-transport-core/core"
)

//go:embed static
var Static fs.FS

//go:embed translations
var Translate fs.FS

//go:embed templates
var Templates fs.FS

func main() {
	staticFS, err := fs.Sub(Static, "static")
	if err != nil {
		panic(err)
	}

	translateFS, err := fs.Sub(Translate, "translate")
	if err != nil {
		panic(err)
	}

	templatesFS, err := fs.Sub(Templates, "templates")
	if err != nil {
		panic(err)
	}
	
    app := core.New()
    app.Config = core.NewConfig("config.yml")
    app.DefaultError = "unknown_error"

    // Now translations will be loaded from embedded files in Go program
    app.TranslationsFS = translateFS
    app.PreloadLanguages = core.DefaultLanguages
    
    app.ConfigureRouter(func(engine *gin.Engine) {
    	// fs.FS should be converted to the http.FileSystem
		
    	// FS implements the io/fs package's FS interface,
    	// so it can be used with any package that understands file systems,
    	// including net/http, text/template, and html/template.
        engine.StaticFS("/static", http.FS(staticFS))
        engine.HTMLRender = app.CreateRendererFS(
			templatesFS,
            func(renderer *core.Renderer) {
                // Same Push method here, but without relative directory.
                r.Push("home", "layout.html", "home.html")
            }, 
            template.FuncMap{},
        )
    })
    
    if err := app.Prepare().Run(); err != nil {
        fmt.Printf("Fatal error: %s", err.Error())
        os.Exit(1)
    }
}
```
### Migration generator
This library contains helper tool for transports. You can install it via go:
```sh
$ go get -u github.com/retailcrm/mg-transport-core/cmd/transport-core-tool
```
Currently, it only can generate new migrations for your transport.
