package core

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"testing"

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
	renderer Renderer
}

func (t *TemplateTest) SetupSuite() {
	t.initTestData()
	t.renderer = t.initRenderer()
}

func (t *TemplateTest) initTestData() {
	if _, err := os.Stat(testTemplatesDir); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(testTemplatesDir, os.ModePerm)
		require.Nil(t.T(), err)
		data1 := []byte(`data {{template "body" .}}`)
		data2 := []byte(`{{define "body"}}test {{"test" | trans}}{{end}}`)
		err1 := ioutil.WriteFile(fmt.Sprintf(testTemplatesFile, 1), data1, os.ModePerm)
		err2 := ioutil.WriteFile(fmt.Sprintf(testTemplatesFile, 2), data2, os.ModePerm)
		require.Nil(t.T(), err1)
		require.Nil(t.T(), err2)
	}
}

func (t *TemplateTest) initRenderer() Renderer {
	return NewRenderer(template.FuncMap{
		"trans": func(data string) string {
			if data == "test" {
				return "ok"
			}

			return "fail"
		},
	})
}

func (t *TemplateTest) Test_Push() {
	tpl := t.renderer.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tpl.Templates()))
}

func (t *TemplateTest) Test_PushAlreadyExists() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	tpl := t.renderer.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tpl.Templates()))
}

func (t *TemplateTest) Test_PushNewInstance() {
	defer func() {
		assert.Nil(t.T(), recover())
	}()

	newInstance := t.initRenderer()
	tpl := newInstance.Push("index", fmt.Sprintf(testTemplatesFile, 1), fmt.Sprintf(testTemplatesFile, 2))
	assert.Equal(t.T(), 3, len(tpl.Templates()))
}

func TestTemplate_Suite(t *testing.T) {
	suite.Run(t, new(TemplateTest))
}
