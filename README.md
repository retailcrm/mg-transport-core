## Message Gateway Transport Library
[![Build Status](https://github.com/retailcrm/mg-transport-core/workflows/ci/badge.svg)](https://github.com/retailcrm/mg-transport-core/actions?query=workflow%3Aci)
[![codecov](https://codecov.io/gh/retailcrm/mg-transport-core/branch/master/graph/badge.svg)](https://codecov.io/gh/retailcrm/mg-transport-core)
[![pkg.go.dev](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/retailcrm/mg-transport-core/core)
[![Go Report Card](https://goreportcard.com/badge/github.com/retailcrm/mg-transport-core)](https://goreportcard.com/report/github.com/retailcrm/mg-transport-core)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/retailcrm/mg-transport-core/blob/master/LICENSE.md)    

This library provides different functions like error-reporting, logging, localization, etc. in order to make it easier to create transports.   
Usage:
```go
package main

import (
    "os"
    "fmt"
    "html/template"

    "github.com/gin-gonic/gin"
    "github.com/gobuffalo/packr/v2"
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
[packr](https://github.com/gobuffalo/packr/tree/master/v2) can be used to provide resource embedding. In order to use packr you must follow
[this instruction](https://github.com/gobuffalo/packr/tree/master/v2#library-installation), and provide boxes with templates,
translations and assets to library. Example:
```go
package main

import (
    "os"
    "fmt"
    "html/template"

    "github.com/gin-gonic/gin"
    "github.com/gobuffalo/packr/v2"
    "github.com/retailcrm/mg-transport-core/core"
)

func main() {
    static := packr.New("assets", "./static")
    templates := packr.New("templates", "./templates")
    translations := packr.New("translations", "./translate")
    
    app := core.New()
    app.Config = core.NewConfig("config.yml")
    app.DefaultError = "unknown_error"

    // Now translations will be loaded from packr.Box
    app.TranslationsBox = translations
    app.PreloadLanguages = core.DefaultLanguages
    
    app.ConfigureRouter(func(engine *gin.Engine) {
        // gin.Engine can use packr.Box as http.FileSystem
        engine.StaticFS("/static", static)
        engine.HTMLRender = app.CreateRendererFS(
            templates, 
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
