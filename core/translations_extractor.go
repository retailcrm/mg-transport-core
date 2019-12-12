package core

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"gopkg.in/yaml.v2"
)

// TranslationsExtractor is a tool to load raw translations data from files or from box.
// It is feasible to be used in tests in order to check translation files correctness.
// The easiest way to check correctness is to check keys in translations.
// TranslationsExtractor IS NOT supposed to check correctness of translations - it's just an extractor.
// Translations can be checked manually, or via external library like https://github.com/google/go-cmp
type TranslationsExtractor struct {
	fileNameTemplate string
	TranslationsBox  *packr.Box
	TranslationsPath string
}

// NewTranslationsExtractor constructor. Use "translate.{}.yml" as template if your translations are named like "translate.en.yml"
func NewTranslationsExtractor(fileNameTemplate string) *TranslationsExtractor {
	return &TranslationsExtractor{fileNameTemplate: fileNameTemplate}
}

// unmarshalToMap returns map with unmarshaled data or error
func (t *TranslationsExtractor) unmarshalToMap(in []byte) (map[string]interface{}, error) {
	var dataMap map[string]interface{}

	if err := yaml.Unmarshal(in, &dataMap); err == nil {
		return dataMap, nil
	} else {
		return dataMap, err
	}
}

// loadYAMLBox loads YAML from box
func (t *TranslationsExtractor) loadYAMLBox(fileName string) (map[string]interface{}, error) {
	var dataMap map[string]interface{}

	if data, err := t.TranslationsBox.Find(fileName); err != nil {
		return dataMap, err
	} else {
		return t.unmarshalToMap(data)
	}
}

// loadYAMLFile loads YAML from file
func (t *TranslationsExtractor) loadYAMLFile(fileName string) (map[string]interface{}, error) {
	var dataMap map[string]interface{}

	if info, err := os.Stat(fileName); err == nil {
		if !info.IsDir() {
			if path, err := filepath.Abs(fileName); err == nil {
				if source, err := ioutil.ReadFile(path); err != nil {
					return dataMap, err
				} else {
					return t.unmarshalToMap(source)
				}
			} else {
				return dataMap, err
			}
		} else {
			return dataMap, errors.New("directory provided instead of file")
		}
	} else {
		return dataMap, err
	}
}

// loadYAML loads YAML from filesystem or from packr box - depends on what was configured. Can return error.
func (t *TranslationsExtractor) loadYAML(fileName string) (map[string]interface{}, error) {
	if t.TranslationsBox != nil {
		return t.loadYAMLBox(fileName)
	} else if t.TranslationsPath != "" {
		return t.loadYAMLFile(filepath.Join(t.TranslationsPath, fileName))
	} else {
		return map[string]interface{}{}, errors.New("nor box nor translations directory was provided")
	}
}

// GetMapKeys returns sorted map keys from map[string]interface{} - useful to check keys in several translation files
func (t *TranslationsExtractor) GetMapKeys(data map[string]interface{}) []string {
	keys := make([]string, len(data))

	i := 0
	for k := range data {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	return keys
}

// LoadLocale returns translation file data with provided locale
func (t *TranslationsExtractor) LoadLocale(locale string) (map[string]interface{}, error) {
	return t.loadYAML(strings.Replace(t.fileNameTemplate, "{}", locale, 1))
}

// LoadLocaleKeys returns only sorted keys from translation file
func (t *TranslationsExtractor) LoadLocaleKeys(locale string) ([]string, error) {
	if data, err := t.LoadLocale(locale); err != nil {
		return []string{}, err
	}

	return t.GetMapKeys(data), nil
}
