package core

import (
	"html/template"
	"io/ioutil"
	"path"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

// DefaultLanguages for transports
var DefaultLanguages = []language.Tag{
	language.English,
	language.Russian,
	language.Spanish,
}

// DefaultLanguage is a base language which will be chosen if current language is unspecified
var DefaultLanguage = language.English

// LocalizerContextKey is a key which is used to store localizer in gin.Context key-value storage
const LocalizerContextKey = "localizer"

// Localizer struct
type Localizer struct {
	i18nStorage      *sync.Map
	TranslationsBox  *packr.Box
	loadMutex        *sync.RWMutex
	LocaleMatcher    language.Matcher
	LanguageTag      language.Tag
	TranslationsPath string
}

// NewLocalizer returns localizer instance with specified parameters.
// Usage:
//      NewLocalizer(language.English, DefaultLocalizerMatcher(), "translations")
func NewLocalizer(locale language.Tag, matcher language.Matcher, translationsPath string) *Localizer {
	localizer := &Localizer{
		i18nStorage:      &sync.Map{},
		LocaleMatcher:    matcher,
		TranslationsPath: translationsPath,
		loadMutex:        &sync.RWMutex{},
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
		i18nStorage:     &sync.Map{},
		LocaleMatcher:   matcher,
		TranslationsBox: translationsBox,
		loadMutex:       &sync.RWMutex{},
	}
	localizer.SetLanguage(locale)
	localizer.LoadTranslations()

	return localizer
}

// DefaultLocalizerBundle returns new localizer bundle with English as default language
func DefaultLocalizerBundle() *i18n.Bundle {
	return i18n.NewBundle(DefaultLanguage)
}

// LocalizerBundle returns new localizer bundle provided language as default
func LocalizerBundle(tag language.Tag) *i18n.Bundle {
	return i18n.NewBundle(tag)
}

// DefaultLocalizerMatcher returns matcher with English, Russian and Spanish tags
func DefaultLocalizerMatcher() language.Matcher {
	return language.NewMatcher(DefaultLanguages)
}

// Clone *core.Localizer. Clone shares it's translations with the parent localizer. Language tag will not be shared.
// Because of that you can change clone's language without affecting parent localizer.
// This method should be used when LocalizationMiddleware is not feasible (outside of *gin.HandlerFunc).
func (l *Localizer) Clone() *Localizer {
	clone := &Localizer{
		i18nStorage:      l.i18nStorage,
		TranslationsBox:  l.TranslationsBox,
		LocaleMatcher:    l.LocaleMatcher,
		LanguageTag:      l.LanguageTag,
		TranslationsPath: l.TranslationsPath,
		loadMutex:        l.loadMutex,
	}
	clone.SetLanguage(DefaultLanguage)

	return clone
}

// LocalizationMiddleware returns gin.HandlerFunc which will set localizer language by Accept-Language header
// Result Localizer instance will share it's internal data (translations, bundles, etc) with instance which was used
// to append middleware to gin.
// Because of that all Localizer instances from this middleware will share *same* mutex. This mutex is used to wrap
// i18n.Bundle methods (those aren't goroutine-safe to use).
// Usage:
//      engine := gin.New()
//      localizer := NewLocalizer("en", DefaultLocalizerMatcher(), "translations")
//      engine.Use(localizer.LocalizationMiddleware())
func (l *Localizer) LocalizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clone := l.Clone()
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

// getLocaleBundle returns current locale bundle and creates it if needed
func (l *Localizer) getLocaleBundle() *i18n.Bundle {
	return l.createLocaleBundleByTag(l.LanguageTag)
}

// createLocaleBundleByTag creates locale bundle by language tag
func (l *Localizer) createLocaleBundleByTag(tag language.Tag) *i18n.Bundle {
	bundle := i18n.NewBundle(tag)
	bundle.RegisterUnmarshalFunc("yml", yaml.Unmarshal)
	l.loadTranslationsToBundle(bundle)

	return bundle
}

// LoadTranslations will load all translation files from translations directory or from embedded box
func (l *Localizer) LoadTranslations() {
	defer l.loadMutex.Unlock()
	l.loadMutex.Lock()
	l.getCurrentLocalizer()
}

// loadTranslationsToBundle loads translations to provided bundle
func (l *Localizer) loadTranslationsToBundle(i18nBundle *i18n.Bundle) {
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
	var localizer *i18n.Localizer

	if l.isUnd(tag) {
		tag = DefaultLanguage
	}

	if item, ok := l.i18nStorage.Load(tag); !ok {
		l.i18nStorage.Store(tag, i18n.NewLocalizer(l.createLocaleBundleByTag(tag), tag.String()))
	} else {
		localizer = item.(*i18n.Localizer)
	}

	return localizer
}

func (l *Localizer) matchByString(al string) language.Tag {
	tag, _ := language.MatchStrings(l.LocaleMatcher, al)
	if l.isUnd(tag) {
		return DefaultLanguage
	}

	return tag
}

func (l *Localizer) isUnd(tag language.Tag) bool {
	return tag == language.Und || tag.IsRoot()
}

// getCurrentLocalizer returns *i18n.Localizer with current language tag
func (l *Localizer) getCurrentLocalizer() *i18n.Localizer {
	return l.getLocalizer(l.LanguageTag)
}

// SetLocale will change language for current localizer
func (l *Localizer) SetLocale(al string) {
	l.SetLanguage(l.matchByString(al))
}

// Preload provided languages (so they will not be loaded every time in middleware)
func (l *Localizer) Preload(tags []language.Tag) {
	for _, tag := range tags {
		l.getLocalizer(tag)
	}
}

// SetLanguage will change language using language tag
func (l *Localizer) SetLanguage(tag language.Tag) {
	if l.isUnd(tag) {
		tag = DefaultLanguage
	}

	l.LanguageTag = tag
	l.LoadTranslations()
}

// FetchLanguage will load language from tag
//
// Deprecated: Use `(*core.Localizer).LoadTranslations()` instead
func (l *Localizer) FetchLanguage() {
	l.LoadTranslations()
}

// GetLocalizedMessage will return localized message by it's ID. It doesn't use `Must` prefix in order to keep BC.
func (l *Localizer) GetLocalizedMessage(messageID string) string {
	return l.getCurrentLocalizer().MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
}

// GetLocalizedTemplateMessage will return localized message with specified data. It doesn't use `Must` prefix in order to keep BC.
// It uses text/template syntax: https://golang.org/pkg/text/template/
func (l *Localizer) GetLocalizedTemplateMessage(messageID string, templateData map[string]interface{}) string {
	return l.getCurrentLocalizer().MustLocalize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
}

// Localize will return localized message by it's ID, or error if message wasn't found
func (l *Localizer) Localize(messageID string) (string, error) {
	return l.getCurrentLocalizer().Localize(&i18n.LocalizeConfig{MessageID: messageID})
}

// LocalizeTemplateMessage will return localized message with specified data, or error if message wasn't found
// It uses text/template syntax: https://golang.org/pkg/text/template/
func (l *Localizer) LocalizeTemplateMessage(messageID string, templateData map[string]interface{}) (string, error) {
	return l.getCurrentLocalizer().Localize(&i18n.LocalizeConfig{
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
		}
	}

	return nil, false
}

// MustGetContextLocalizer returns Localizer instance if it exists in provided context. Panics otherwise.
func MustGetContextLocalizer(c *gin.Context) *Localizer {
	if localizer, ok := GetContextLocalizer(c); ok {
		return localizer
	}
	panic("localizer is not present in provided context")
}
