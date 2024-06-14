package core

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/language"

	"github.com/retailcrm/mg-transport-core/v2/core/util/errorutil"
)

var (
	testTranslationsDir = path.Join(os.TempDir(), "translations_test_dir")
	testLangFiles       = map[string][]byte{
		"translate.en.yml": []byte("message: Test message\nmessage_template: Test message with {{.data}}"),
		"translate.es.yml": []byte("message: Mensaje de prueba\nmessage_template: Mensaje de prueba con {{.data}}"),
		"translate.ru.yml": []byte("message: Тестовое сообщение\nmessage_template: Тестовое сообщение с {{.data}}"),
	}
)

func createTestLangFiles(t *testing.T) {
	for name, data := range testLangFiles {
		fileName := path.Join(testTranslationsDir, name)

		if _, err := os.Stat(testTranslationsDir); err != nil && os.IsNotExist(err) {
			err := os.Mkdir(testTranslationsDir, os.ModePerm)
			require.Nil(t, err)
		}

		if _, err := os.Stat(fileName); err != nil && os.IsNotExist(err) {
			err = os.WriteFile(fileName, data, os.ModePerm)
			require.Nil(t, err)
		}
	}
}

type LocalizerTest struct {
	suite.Suite
	localizer LocalizerInterface
}

func (l *LocalizerTest) SetupSuite() {
	createTestLangFiles(l.T())
	l.localizer = NewLocalizer(language.English, DefaultLocalizerMatcher(), testTranslationsDir)
}

func (l *LocalizerTest) Test_SetLocale() {
	defer func() {
		require.Nil(l.T(), recover())
	}()

	l.localizer.SetLocale("es")
	assert.Equal(l.T(), "Mensaje de prueba", l.localizer.GetLocalizedMessage("message"))
	l.localizer.SetLocale("en")
	assert.Equal(l.T(), "Test message", l.localizer.GetLocalizedMessage("message"))
}

func (l *LocalizerTest) Test_LocalizationMiddleware_Context() {
	l.localizer.Preload(DefaultLanguages)
	middlewareFunc := l.localizer.LocalizationMiddleware()
	require.NotNil(l.T(), middlewareFunc)

	enContext := l.getContextWithLang(language.English)
	esContext := l.getContextWithLang(language.Spanish)
	ruContext := l.getContextWithLang(language.Russian)

	middlewareFunc(enContext)
	middlewareFunc(esContext)
	middlewareFunc(ruContext)

	defer func() {
		assert.Nil(l.T(), recover())
	}()

	enLocalizer := MustGetContextLocalizer(enContext)
	esLocalizer := MustGetContextLocalizer(esContext)
	ruLocalizer := MustGetContextLocalizer(ruContext)

	assert.NotNil(l.T(), enLocalizer)
	assert.NotNil(l.T(), esLocalizer)
	assert.NotNil(l.T(), ruLocalizer)

	assert.Equal(l.T(), language.English, enLocalizer.Language())
	assert.Equal(l.T(), language.Spanish, esLocalizer.Language())
	assert.Equal(l.T(), language.Russian, ruLocalizer.Language())

	assert.Equal(l.T(), "Test message", enLocalizer.GetLocalizedMessage("message"))
	assert.Equal(l.T(), "Mensaje de prueba", esLocalizer.GetLocalizedMessage("message"))
	assert.Equal(l.T(), "Тестовое сообщение", ruLocalizer.GetLocalizedMessage("message"))
}

func (l *LocalizerTest) Test_LocalizationMiddleware_Httptest() {
	var wg sync.WaitGroup
	l.localizer.Preload(DefaultLanguages)
	langMsgMap := map[language.Tag]string{
		language.English: "Test message",
		language.Russian: "Тестовое сообщение",
		language.Spanish: "Mensaje de prueba",
	}

	fw := gin.New()
	fw.Use(l.localizer.LocalizationMiddleware())
	fw.GET("/test", func(c *gin.Context) {
		loc := MustGetContextLocalizer(c)
		c.String(http.StatusOK, loc.GetLocalizedMessage("message"))
	})

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		i := i
		go func(m map[language.Tag]string, wg *sync.WaitGroup) {
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(i))) // nolint:gosec
			var tag language.Tag
			switch r.Intn(3-1) + 1 { // nolint:gosec
			case 1:
				tag = language.English
			case 2:
				tag = language.Russian
			case 3:
				tag = language.Spanish
			}

			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			require.NoError(l.T(), err)
			req.Header.Add("Accept-Language", tag.String())
			rr := httptest.NewRecorder()
			fw.ServeHTTP(rr, req)
			assert.Equal(l.T(), m[tag], rr.Body.String())
			wg.Done()
		}(langMsgMap, &wg)
	}

	wg.Wait()
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

func (l *LocalizerTest) Test_Clone() {
	defer func() {
		require.Nil(l.T(), recover())
	}()

	localizer := l.localizer.Clone().(LocalizerInterface)
	localizer.SetLanguage(language.Russian)

	assert.NotEqual(l.T(), l.localizer.Language(), localizer.Language())
	assert.Equal(l.T(), "Test message", l.localizer.GetLocalizedMessage("message"))
	assert.Equal(l.T(), "Тестовое сообщение", localizer.GetLocalizedMessage("message"))
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
	assert.Equal(l.T(), "Test message", resp.(errorutil.Response).Error)
}

// getContextWithLang generates context with Accept-Language header.
func (l *LocalizerTest) getContextWithLang(tag language.Tag) *gin.Context {
	urlInstance, _ := url.Parse("https://example.com")
	headers := http.Header{}
	headers.Add("Accept-Language", tag.String())
	return &gin.Context{
		Request: &http.Request{
			Method: "GET",
			URL:    urlInstance,
			Proto:  "https",
			Header: headers,
			Host:   "example.com",
		},
		Keys: map[string]interface{}{},
	}
}

func (l *LocalizerTest) TearDownSuite() {
	err := os.RemoveAll(testTranslationsDir)
	require.Nil(l.T(), err)
}

func TestLocalizer_Suite(t *testing.T) {
	suite.Run(t, new(LocalizerTest))
}

func TestLocalizer_DefaultLocalizerMatcher(t *testing.T) {
	assert.NotNil(t, DefaultLocalizerMatcher())
}

func TestLocalizer_DefaultLocalizerBundle(t *testing.T) {
	assert.NotNil(t, DefaultLocalizerBundle())
}

func TestLocalizer_LocalizerBundle(t *testing.T) {
	assert.NotNil(t, LocalizerBundle(language.Russian))
}

func TestLocalizer_NoDirectory(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()

	_ = NewLocalizer(
		language.English,
		DefaultLocalizerMatcher(),
		path.Join(os.TempDir(), "this directory should not exist"),
	)
}
