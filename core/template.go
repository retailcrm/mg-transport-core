package core

import (
	"html/template"

	"github.com/gin-contrib/multitemplate"
)

// Renderer wraps multitemplate.Renderer in order to make it easier to use
type Renderer struct {
	multitemplate.Renderer
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
	return r.AddFromFilesFuncs(name, r.FuncMap, files...)
}
