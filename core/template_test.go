package core

import (
	"fmt"
	"html/template"
	"os"
	"path"
	"testing"

	"github.com/gin-contrib/multitemplate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	testTemplatesDir  = path.Join(os.TempDir(), "templates_test_dir")
	testTemplatesFile = path.Join(testTemplatesDir, "tpl%d.html")
)

type TemplateTest struct {
	suite.Suite
	static  Renderer
	dynamic Renderer
}

func (t *TemplateTest) SetupSuite() {
	t.initTestData()
	t.static = t.initStatic()
	t.dynamic = t.initDynamic()
}

func (t *TemplateTest) initTestData() {
	if _, err := os.Stat(testTemplatesDir); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(testTemplatesDir, os.ModePerm)
		require.Nil(t.T(), err)
		data1 := []byte(`data {{template "body" .}}`)
		data2 := []byte(`{{define "body"}}test {{"test" | trans}}{{end}}`)
		err1 := os.WriteFile(fmt.Sprintf(testTemplatesFile, 1), data1, os.ModePerm)
		err2 := os.WriteFile(fmt.Sprintf(testTemplatesFile, 2), data2, os.ModePerm)
		require.Nil(t.T(), err1)
		require.Nil(t.T(), err2)
	}
}

func (t *TemplateTest) initStatic() Renderer {
	return NewStaticRenderer(template.FuncMap{
		"trans": func(data string) string {
			if data == "test" {
				return "ok"
			}

			return "fail"
		},
	})
}

func (t *TemplateTest) initDynamic() Renderer {
	return NewDynamicRenderer(template.FuncMap{
		"trans": func(data string) string {
			if data == "test" {
				return "ok"
			}

			return "fail"
		},
	})
}

func (t *TemplateTest) Test_Push() {
	tplStatic := t.static.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	tplDynamic := t.dynamic.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tplStatic.Templates()))
	assert.Equal(t.T(), 3, len(tplDynamic.Templates()))
}

func (t *TemplateTest) Test_PushAlreadyExists() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	tplStatic := t.static.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	tplDynamic := t.dynamic.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tplStatic.Templates()))
	assert.Equal(t.T(), 3, len(tplDynamic.Templates()))
}

func (t *TemplateTest) Test_PushNewInstanceStatic() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	newInstance := t.initStatic()
	tpl := newInstance.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tpl.Templates()))
}

func (t *TemplateTest) Test_PushNewInstanceDynamic() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	newInstance := t.initDynamic()
	tpl := newInstance.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tpl.Templates()))
}

func TestTemplate_NewRenderer(t *testing.T) {
	r := NewRenderer(template.FuncMap{})
	assert.NotNil(t, r)
}

func TestTemplate_NewStaticRenderer(t *testing.T) {
	r := NewStaticRenderer(template.FuncMap{})
	assert.NotNil(t, r)
	assert.IsType(t, multitemplate.New(), r.Renderer)
}

func TestTemplate_NewDynamicRenderer(t *testing.T) {
	r := NewDynamicRenderer(template.FuncMap{})
	assert.NotNil(t, r)
	assert.IsType(t, multitemplate.NewDynamic(), r.Renderer)
}

func TestTemplate_Suite(t *testing.T) {
	suite.Run(t, new(TemplateTest))
}
