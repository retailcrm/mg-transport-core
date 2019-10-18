package core

import (
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"
)

var (
	testTranslationsDir = path.Join(os.TempDir(), "translations_test_dir")
	testLangFile        = path.Join(testTranslationsDir, "translate.en.yml")
)

type LocalizerTest struct {
	suite.Suite
	localizer *Localizer
}

func (l *LocalizerTest) SetupSuite() {
	if _, err := os.Stat(testTranslationsDir); err != nil && os.IsNotExist(err) {
		err := os.Mkdir(testTranslationsDir, os.ModePerm)
		require.Nil(l.T(), err)
		data := []byte("message: Test message\nmessage_template: Test message with {{.data}}")
		err = ioutil.WriteFile(testLangFile, data, os.ModePerm)
		require.Nil(l.T(), err)
	}

	l.localizer = NewLocalizer(language.English, DefaultLocalizerBundle(), DefaultLocalizerMatcher(), testTranslationsDir)
}

func (l *LocalizerTest) Test_SetLocale() {
	defer func() {
		require.Nil(l.T(), recover())
	}()

	l.localizer.SetLocale("en")
}

func (l *LocalizerTest) Test_LocalizationMiddleware() {
	assert.NotNil(l.T(), l.localizer.LocalizationMiddleware())
}

func (l *LocalizerTest) Test_LocalizationFuncMap() {
	functions := l.localizer.LocalizationFuncMap()
	_, ok := functions["trans"]
	assert.True(l.T(), ok)
}

func (l *LocalizerTest) Test_GetLocalizedMessage() {
	defer func() {
		require.Nil(l.T(), recover())
	}()

	message := l.localizer.GetLocalizedMessage("message")
	assert.Equal(l.T(), "Test message", message)
}

func (l *LocalizerTest) Test_GetLocalizedTemplateMessage() {
	defer func() {
		require.Nil(l.T(), recover())
	}()

	message := l.localizer.GetLocalizedTemplateMessage("message_template", map[string]interface{}{"data": "template"})
	assert.Equal(l.T(), "Test message with template", message)
}

func (l *LocalizerTest) Test_BadRequestLocalized() {
	status, resp := l.localizer.BadRequestLocalized("message")

	assert.Equal(l.T(), http.StatusBadRequest, status)
	assert.Equal(l.T(), "Test message", resp.(ErrorResponse).Error)
}

func (l *LocalizerTest) TearDownSuite() {
	err := os.RemoveAll(testTranslationsDir)
	require.Nil(l.T(), err)
}

func TestLocalizer_Suite(t *testing.T) {
	suite.Run(t, new(LocalizerTest))
}

func TestLocalizer_NoDirectory(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	_ = NewLocalizer(
		language.English,
		DefaultLocalizerBundle(),
		DefaultLocalizerMatcher(),
		path.Join(os.TempDir(), "this directory should not exist"),
	)
}
