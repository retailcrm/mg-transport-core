package core

import (
	"html/template"
	"io/ioutil"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

// Localizer struct
type Localizer struct {
	i18n             *i18n.Localizer
	TranslationsBox  *packr.Box
	LocaleBundle     *i18n.Bundle
	LocaleMatcher    language.Matcher
	LanguageTag      language.Tag
	TranslationsPath string
}

// NewLocalizer returns localizer instance with specified parameters.
// Usage:
//      NewLocalizer(language.English, DefaultLocalizerBundle(), DefaultLocalizerMatcher(), "translations")
func NewLocalizer(locale language.Tag, bundle *i18n.Bundle, matcher language.Matcher, translationsPath string) *Localizer {
	localizer := &Localizer{
		i18n:             nil,
		LocaleBundle:     bundle,
		LocaleMatcher:    matcher,
		TranslationsPath: translationsPath,
	}
	localizer.SetLanguage(locale)
	localizer.LoadTranslations()

	return localizer
}

// DefaultLocalizerBundle returns new localizer bundle with English as default language
func DefaultLocalizerBundle() *i18n.Bundle {
	return i18n.NewBundle(language.English)
}

// DefaultLocalizerMatcher returns matcher with English, Russian and Spanish tags
func DefaultLocalizerMatcher() language.Matcher {
	return language.NewMatcher([]language.Tag{
		language.English,
		language.Russian,
		language.Spanish,
	})
}

// LocalizationMiddleware returns gin.HandlerFunc which will set localizer language by Accept-Language header
// Usage:
//      engine := gin.New()
//      localizer := NewLocalizer("en", DefaultLocalizerBundle(), DefaultLocalizerMatcher(), "translations")
//      engine.Use(localizer.LocalizationMiddleware())
func (l *Localizer) LocalizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		l.SetLocale(c.GetHeader("Accept-Language"))
	}
}

// LocalizationFuncMap returns template.FuncMap (html template is used) with one method - trans
// Usage in code:
//      engine := gin.New()
//      engine.FuncMap = localizer.LocalizationFuncMap()
// or (with multitemplate)
//      renderer := multitemplate.NewRenderer()
//      funcMap := localizer.LocalizationFuncMap()
//      renderer.AddFromFilesFuncs("index", funcMap, "template/index.html")
// funcMap must be passed for every .AddFromFilesFuncs call
// Usage in templates:
//      <p class="info">{{"need_login_msg" | trans}}
// You can borrow FuncMap from this method and add your functions to it.
func (l *Localizer) LocalizationFuncMap() template.FuncMap {
	return template.FuncMap{
		"trans": l.GetLocalizedMessage,
	}
}

func (l *Localizer) getLocaleBundle() *i18n.Bundle {
	if l.LocaleBundle == nil {
		l.LocaleBundle = DefaultLocalizerBundle()
	}
	return l.LocaleBundle
}

// LoadTranslations will load all translation files from translations directory or from embedded box
func (l *Localizer) LoadTranslations() {
	l.getLocaleBundle().RegisterUnmarshalFunc("yml", yaml.Unmarshal)

	switch {
	case l.TranslationsPath != "":
		if err := l.loadFromDirectory(); err != nil {
			panic(err.Error())
		}
	case l.TranslationsBox != nil:
		if err := l.loadFromFS(); err != nil {
			panic(err.Error())
		}
	default:
		panic("TranslationsPath or TranslationsBox should be specified")
	}
}

// LoadTranslations will load all translation files from translations directory
func (l *Localizer) loadFromDirectory() error {
	files, err := ioutil.ReadDir(l.TranslationsPath)
	if err != nil {
		return err
	}

	for _, f := range files {
		if !f.IsDir() {
			l.getLocaleBundle().MustLoadMessageFile(path.Join(l.TranslationsPath, f.Name()))
		}
	}

	return nil
}

// LoadTranslations will load all translation files from embedded box
func (l *Localizer) loadFromFS() error {
	err := l.TranslationsBox.Walk(func(s string, file packd.File) error {
		if fileInfo, err := file.FileInfo(); err == nil {
			if !fileInfo.IsDir() {
				if data, err := ioutil.ReadAll(file); err == nil {
					if _, err := l.getLocaleBundle().ParseMessageFileBytes(data, fileInfo.Name()); err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	} else {
		return nil
	}
}

// SetLocale will change language for current localizer
func (l *Localizer) SetLocale(al string) {
	tag, _ := language.MatchStrings(l.LocaleMatcher, al)
	l.SetLanguage(tag)
}

// SetLanguage will change language using language tag
func (l *Localizer) SetLanguage(tag language.Tag) {
	l.LanguageTag = tag
	l.FetchLanguage()
}

// FetchLanguage will load language from tag
func (l *Localizer) FetchLanguage() {
	l.i18n = i18n.NewLocalizer(l.getLocaleBundle(), l.LanguageTag.String())
}

// GetLocalizedMessage will return localized message by it's ID
func (l *Localizer) GetLocalizedMessage(messageID string) string {
	return l.i18n.MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
}

// GetLocalizedTemplateMessage will return localized message with specified data
// It uses text/template syntax: https://golang.org/pkg/text/template/
func (l *Localizer) GetLocalizedTemplateMessage(messageID string, templateData map[string]interface{}) string {
	return l.i18n.MustLocalize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
}

// BadRequestLocalized is same as BadRequest(string), but passed string will be localized
func (l *Localizer) BadRequestLocalized(err string) (int, interface{}) {
	return BadRequest(l.GetLocalizedMessage(err))
}
