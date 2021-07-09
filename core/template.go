package core

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/gin-contrib/multitemplate"
)

// Renderer wraps multitemplate.Renderer in order to make it easier to use.
type Renderer struct {
	multitemplate.Renderer
	TemplatesFS  embed.FS
	TemplatesDir string
	FuncMap      template.FuncMap
	alreadyAdded map[string]*template.Template
}

// NewRenderer is a Renderer constructor.
func NewRenderer(funcMap template.FuncMap) Renderer {
	return newRendererWithMultitemplate(funcMap, multitemplate.NewRenderer())
}

// NewStaticRenderer is a Renderer constructor with multitemplate.Render.
func NewStaticRenderer(funcMap template.FuncMap) Renderer {
	return newRendererWithMultitemplate(funcMap, multitemplate.New())
}

// NewDynamicRenderer is a Renderer constructor with multitemplate.DynamicRender.
func NewDynamicRenderer(funcMap template.FuncMap) Renderer {
	return newRendererWithMultitemplate(funcMap, multitemplate.NewDynamic())
}

// newRendererWithMultitemplate initializes Renderer with provided multitemplate.Renderer instance.
func newRendererWithMultitemplate(funcMap template.FuncMap, renderer multitemplate.Renderer) Renderer {
	return Renderer{
		Renderer:     renderer,
		FuncMap:      funcMap,
		alreadyAdded: map[string]*template.Template{},
	}
}

// Push is an AddFromFilesFuncs wrapper.
func (r *Renderer) Push(name string, files ...string) *template.Template {
	if tpl := r.getTemplate(name); tpl != nil {
		return tpl
	}

	if _, err := r.TemplatesFS.ReadDir(r.TemplatesDir); err == nil {
		return r.storeTemplate(name, r.addFromFS(name, r.FuncMap, files...))
	}

	return r.storeTemplate(name, r.AddFromFilesFuncs(name, r.FuncMap, files...))
}

// addFromFS adds embedded template.
func (r *Renderer) addFromFS(name string, funcMap template.FuncMap, files ...string) *template.Template {
	var filesData []string

	for _, fileName := range files {
		if data, err := r.TemplatesFS.ReadFile(fmt.Sprintf("%s/%s", r.TemplatesDir, fileName)); err == nil {
			filesData = append(filesData, string(data))
		}
	}

	return r.AddFromStringsFuncs(name, funcMap, filesData...)
}

// storeTemplate stores built template if multitemplate.DynamicRender is used.
// Dynamic render doesn't store templates - it stores builders, that's why we can't just extract them.
// It possibly can cause data inconsistency in developer environments where return value from Renderer.Push is used.
func (r *Renderer) storeTemplate(name string, tpl *template.Template) *template.Template {
	if _, ok := r.Renderer.(multitemplate.DynamicRender); ok {
		r.alreadyAdded[name] = tpl
	}

	return tpl
}

// getTemplate returns template from render or from storage.
func (r *Renderer) getTemplate(name string) *template.Template {
	if renderer, ok := r.Renderer.(multitemplate.Render); ok {
		if i, ok := renderer[name]; ok {
			return i
		}
	}

	if _, ok := r.Renderer.(multitemplate.DynamicRender); ok {
		if i, ok := r.alreadyAdded[name]; ok {
			return i
		}
	}

	return nil
}
