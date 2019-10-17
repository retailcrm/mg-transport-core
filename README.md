## MG Transport Library
[![Build Status](https://travis-ci.org/retailcrm/mg-transport-core.svg?branch=master)](https://travis-ci.org/retailcrm/mg-transport-core)
[![codecov](https://codecov.io/gh/retailcrm/mg-transport-core/branch/master/graph/badge.svg)](https://codecov.io/gh/retailcrm/mg-transport-core)  
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
    app := core.New()
    app.Config = core.NewConfig("config.yml")
    app.DefaultError = "unknown_error"
    app.TranslationsPath = "./translations"
    
    app.ConfigureRouter(func(engine *gin.Engine) {
        engine.Static("/static", "./static")
        engine.HTMLRender = app.CreateRenderer(
            func(renderer *core.Renderer) {
                // insert templates here. Example:
                r.Push("home", "templates/layout.html", "templates/home.html")
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
    app.TranslationsBox = translations
    
    app.ConfigureRouter(func(engine *gin.Engine) {
        engine.StaticFS("/static", static)
        engine.HTMLRender = app.CreateRendererFS(
            templates, 
            func(renderer *core.Renderer) {
                // insert templates here. Example:
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