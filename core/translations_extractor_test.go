package core

import (
	"io/ioutil"
	"os"
	"reflect"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
)

type TranslationsExtractorTest struct {
	suite.Suite
	extractor *TranslationsExtractor
}

func (t *TranslationsExtractorTest) SetupSuite() {
	translation := map[string]string{
		"test":    "first",
		"another": "second",
	}
	data, _ := yaml.Marshal(translation)
	errWrite := ioutil.WriteFile("/tmp/translate.en.yml", data, os.ModePerm)
	require.NoError(t.T(), errWrite)

	t.extractor = NewTranslationsExtractor("translate.{}.yml")
	t.extractor.TranslationsPath = "/tmp"
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
	translation := map[string]string{
		"test":    "first",
		"another": "second",
	}
	data, _ := yaml.Marshal(translation)
	mapData, err := t.extractor.unmarshalToMap(data)
	require.NoError(t.T(), err)

	assert.True(t.T(), reflect.DeepEqual(translation, mapData))
}
