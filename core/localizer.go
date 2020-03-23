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

// LocalizerContextKey is a key which is used to store localizer in gin.Context key-value storage
const LocalizerContextKey = "localizer"

// Localizer struct
type Localizer struct {
	i18nStorage      map[language.Tag]*i18n.Localizer
	bundleStorage    map[language.Tag]*i18n.Bundle
	TranslationsBox  *packr.Box
	LocaleMatcher    language.Matcher
	LanguageTag      language.Tag
	TranslationsPath string
}

// NewLocalizer returns localizer instance with specified parameters.
// Usage:
//      NewLocalizer(language.English, DefaultLocalizerMatcher(), "translations")
func NewLocalizer(locale language.Tag, matcher language.Matcher, translationsPath string) *Localizer {
	localizer := &Localizer{
		i18nStorage:      map[language.Tag]*i18n.Localizer{},
		bundleStorage:    map[language.Tag]*i18n.Bundle{},
		LocaleMatcher:    matcher,
		TranslationsPath: translationsPath,
	}
	localizer.SetLanguage(locale)
	localizer.LoadTranslations()

	return localizer
}

// NewLocalizerFS returns localizer instance with specified parameters. *packr.Box should be used instead of directory.
// Usage:
//      NewLocalizerFS(language.English, DefaultLocalizerMatcher(), translationsBox)
// TODO This code should be covered with tests.
func NewLocalizerFS(locale language.Tag, matcher language.Matcher, translationsBox *packr.Box) *Localizer {
	localizer := &Localizer{
		i18nStorage:     map[language.Tag]*i18n.Localizer{},
		bundleStorage:   map[language.Tag]*i18n.Bundle{},
		LocaleMatcher:   matcher,
		TranslationsBox: translationsBox,
	}
	localizer.SetLanguage(locale)
	localizer.LoadTranslations()

	return localizer
}

// DefaultLocalizerBundle returns new localizer bundle with English as default language
func DefaultLocalizerBundle() *i18n.Bundle {
	return i18n.NewBundle(language.English)
}

// LocalizerBundle returns new localizer bundle provided language as default
func LocalizerBundle(tag language.Tag) *i18n.Bundle {
	return i18n.NewBundle(tag)
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
		clone := &Localizer{
			i18nStorage:      l.i18nStorage,
			bundleStorage:    l.bundleStorage,
			TranslationsBox:  l.TranslationsBox,
			LocaleMatcher:    l.LocaleMatcher,
			LanguageTag:      l.LanguageTag,
			TranslationsPath: l.TranslationsPath,
		}
		clone.SetLocale(c.GetHeader("Accept-Language"))
		c.Set(LocalizerContextKey, clone)
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

// getLocaleBundle returns locale bundle or default locale bundle
func (l *Localizer) getLocaleBundle() *i18n.Bundle {
	if _, ok := l.bundleStorage[l.LanguageTag]; !ok {
		l.LanguageTag = language.English
		l.bundleStorage[language.English] = DefaultLocalizerBundle()
	}

	return l.bundleStorage[l.LanguageTag]
}

// LoadTranslations will load all translation files from translations directory or from embedded box
func (l *Localizer) LoadTranslations() {
	l.getLocaleBundle().RegisterUnmarshalFunc("yml", yaml.Unmarshal)
	l.loadTranslationsToBundle(l.getLocaleBundle())
}

// loadTranslationsToBundle loads translations to provided bundle
func (l *Localizer) loadTranslationsToBundle(i18nBundle *i18n.Bundle) {
	i18nBundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)

	switch {
	case l.TranslationsPath != "":
		if err := l.loadFromDirectory(i18nBundle); err != nil {
			panic(err.Error())
		}
	case l.TranslationsBox != nil:
		if err := l.loadFromFS(i18nBundle); err != nil {
			panic(err.Error())
		}
	default:
		panic("TranslationsPath or TranslationsBox should be specified")
	}
}

// LoadTranslations will load all translation files from translations directory
func (l *Localizer) loadFromDirectory(i18nBundle *i18n.Bundle) error {
	files, err := ioutil.ReadDir(l.TranslationsPath)
	if err != nil {
		return err
	}

	for _, f := range files {
		if !f.IsDir() {
			i18nBundle.MustLoadMessageFile(path.Join(l.TranslationsPath, f.Name()))
		}
	}

	return nil
}

// LoadTranslations will load all translation files from embedded box
func (l *Localizer) loadFromFS(i18nBundle *i18n.Bundle) error {
	err := l.TranslationsBox.Walk(func(s string, file packd.File) error {
		if fileInfo, err := file.FileInfo(); err == nil {
			if !fileInfo.IsDir() {
				if data, err := ioutil.ReadAll(file); err == nil {
					if _, err := i18nBundle.ParseMessageFileBytes(data, fileInfo.Name()); err != nil {
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
	}

	return nil
}

// getLocalizer returns *i18n.Localizer with provided language tag. It will be created if not exist
func (l *Localizer) getLocalizer(tag language.Tag) *i18n.Localizer {
	if _, ok := l.i18nStorage[tag]; !ok {
		if _, ok := l.bundleStorage[tag]; !ok {
			bundle := i18n.NewBundle(tag)
			l.loadTranslationsToBundle(bundle)
			l.bundleStorage[tag] = bundle
		}

		l.i18nStorage[tag] = i18n.NewLocalizer(l.bundleStorage[tag], tag.String())
	}

	return l.i18nStorage[tag]
}

func (l *Localizer) matchByString(al string) language.Tag {
	tag, _ := language.MatchStrings(l.LocaleMatcher, al)
	return tag
}

// getCurrentLocalizer returns *i18n.Localizer with current language tag
func (l *Localizer) getCurrentLocalizer() *i18n.Localizer {
	return l.getLocalizer(l.LanguageTag)
}

// SetLocale will change language for current localizer
func (l *Localizer) SetLocale(al string) {
	l.SetLanguage(l.matchByString(al))
}

// SetLanguage will change language using language tag
func (l *Localizer) SetLanguage(tag language.Tag) {
	l.LanguageTag = tag
	l.FetchLanguage()
}

// FetchLanguage will load language from tag
func (l *Localizer) FetchLanguage() {
	l.getCurrentLocalizer()
}

// GetLocalizedMessage will return localized message by it's ID
func (l *Localizer) GetLocalizedMessage(messageID string) string {
	return l.getCurrentLocalizer().MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
}

// GetLocalizedTemplateMessage will return localized message with specified data
// It uses text/template syntax: https://golang.org/pkg/text/template/
func (l *Localizer) GetLocalizedTemplateMessage(messageID string, templateData map[string]interface{}) string {
	return l.getCurrentLocalizer().MustLocalize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
}

// BadRequestLocalized is same as BadRequest(string), but passed string will be localized
func (l *Localizer) BadRequestLocalized(err string) (int, interface{}) {
	return BadRequest(l.GetLocalizedMessage(err))
}

// GetContextLocalizer returns localizer from context if it exist there
func GetContextLocalizer(c *gin.Context) (*Localizer, bool) {
	if c == nil {
		return nil, false
	}

	if item, ok := c.Get(LocalizerContextKey); ok {
		if localizer, ok := item.(*Localizer); ok {
			return localizer, true
		} else {
			return nil, false
		}
	} else {
		return nil, false
	}
}

// MustGetContextLocalizer returns Localizer instance if it exists in provided context. Panics otherwise.
func MustGetContextLocalizer(c *gin.Context) *Localizer {
	if localizer, ok := GetContextLocalizer(c); ok {
		return localizer
	}
	panic("localizer is not present in provided context")
}
