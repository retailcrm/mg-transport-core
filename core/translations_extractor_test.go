package core

import (
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
)

type TranslationsExtractorTest struct {
	suite.Suite
	extractor *TranslationsExtractor
}

func (t *TranslationsExtractorTest) getKeys(data map[string]interface{}) []string {
	keys := make([]string, len(data))

	i := 0
	for k := range data {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	return keys
}

func (t *TranslationsExtractorTest) SetupSuite() {
	translation := map[string]string{
		"test":    "first",
		"another": "second",
	}
	data, _ := yaml.Marshal(translation)
	// It's not regular temporary file. Little hack in order to test translations extractor.
	// nolint:gosec
	errWrite := ioutil.WriteFile("/tmp/translate.en.yml", data, os.ModePerm)
	require.NoError(t.T(), errWrite)

	t.extractor = NewTranslationsExtractor("translate.{}.yml")
	t.extractor.TranslationsPath = "/tmp"
	t.extractor.TranslationsBox = nil
}

func (t *TranslationsExtractorTest) Test_LoadLocale() {
	data, err := t.extractor.LoadLocale("en")
	require.NoError(t.T(), err)
	assert.Contains(t.T(), data, "test")
}

func (t *TranslationsExtractorTest) Test_GetMapKeys() {
	testMap := map[string]interface{}{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	keys := []string{"a", "b", "c"}
	mapKeys := t.extractor.GetMapKeys(testMap)

	assert.True(t.T(), reflect.DeepEqual(keys, mapKeys))
}

func (t *TranslationsExtractorTest) Test_unmarshalToMap() {
	translation := map[string]interface{}{
		"test":    "first",
		"another": "second",
	}
	data, _ := yaml.Marshal(translation)
	mapData, err := t.extractor.unmarshalToMap(data)
	require.NoError(t.T(), err)

	assert.True(t.T(), reflect.DeepEqual(t.getKeys(translation), t.getKeys(mapData)))
}

func Test_TranslationExtractor(t *testing.T) {
	suite.Run(t, new(TranslationsExtractorTest))
}
