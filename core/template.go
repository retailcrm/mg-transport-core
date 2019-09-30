package core

import (
	"html/template"

	"github.com/gin-contrib/multitemplate"
	"github.com/gobuffalo/packr/v2"
)

// Renderer wraps multitemplate.Renderer in order to make it easier to use
type Renderer struct {
	multitemplate.Renderer
	TemplatesBox *packr.Box
	FuncMap template.FuncMap
}

// NewRenderer is a Renderer constructor
func NewRenderer(funcMap template.FuncMap) Renderer {
	return Renderer{
		Renderer: multitemplate.NewRenderer(),
		FuncMap:  funcMap,
	}
}

// Push is an AddFromFilesFuncs wrapper
func (r *Renderer) Push(name string, files ...string) *template.Template {
	if r.TemplatesBox == nil {
		return r.AddFromFilesFuncs(name, r.FuncMap, files...)
	} else {
		return r.addFromBox(name, r.FuncMap, files...)
	}
}

// addFromBox adds embedded template
func (r *Renderer) addFromBox(name string, funcMap template.FuncMap, files ...string) *template.Template {
	var filesData []string

	for _, file := range files {
		if data, err := r.TemplatesBox.FindString(file); err == nil {
			filesData = append(filesData, data)
		}
	}

	return r.AddFromStringsFuncs(name, funcMap, filesData...)
}