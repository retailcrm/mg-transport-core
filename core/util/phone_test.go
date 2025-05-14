package util

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestParsePhone(t *testing.T) {
	t.Run("russian numbers", func(t *testing.T) {
		n := "+88002541213"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(8002541213), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

		n = "+78002541213"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.NotNil(t, pn)
		assert.Equal(t, uint64(8002541213), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

		n = "89521548787"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(9521548787), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

		n = "+7-900-123-45-67"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(9001234567), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

	})

	t.Run("us numbers", func(t *testing.T) {
		for _, usMask := range UndefinedUSCodes {
			t.Run(fmt.Sprintf("mask %s", usMask), func(t *testing.T) {
				pNumber := usMask + "7043340"
				pPhone, err := ParsePhone(pNumber)
				require.NoError(t, err)
				assert.Equal(t, pNumber[1:], strconv.FormatUint(pPhone.GetNationalNumber(), 10))
				assert.Equal(t, int32(1), pPhone.GetCountryCode())
			})
		}
	})

	t.Run("german numbers", func(t *testing.T) {
		n := "491736276098"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(1736276098), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeDE), pn.GetCountryCode())

		n = "4915229457499"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(15229457499), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeDE), pn.GetCountryCode())
	})

	t.Run("mexican number", func(t *testing.T) {
		n := "5219982418333"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(19982418333), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeMX), pn.GetCountryCode())

		n = "+521 (998) 241 83 33"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(19982418333), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeMX), pn.GetCountryCode())

		n = "529982418333"
		pn, err = ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(19982418333), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeMX), pn.GetCountryCode())
	})

	t.Run("palestine number", func(t *testing.T) {
		n := "970567800663"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(567800663), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodePS), pn.GetCountryCode())
	})

	t.Run("argentine number", func(t *testing.T) {
		n := "5491131157821"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(91131157821), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeAG), pn.GetCountryCode())
	})

	t.Run("uzbekistan number", func(t *testing.T) {
		n := "998882207724"
		pn, err := ParsePhone(n)
		require.NoError(t, err)
		assert.Equal(t, uint64(882207724), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeUZ), pn.GetCountryCode())
	})
}

func TestFormatNumberForWA(t *testing.T) {
	numbers := map[string]string{
		"79040000000":   "+79040000000",
		"491736276098":  "+491736276098",
		"89185553535":   "+79185553535",
		"4915229457499": "+4915229457499",
		"5491131157821": "+5491131157821",
		"541131157821":  "+5491131157821",
		"5219982418333": "+5219982418333",
		"529982418333":  "+5219982418333",
		"14452385043":   "+14452385043",
		"19452090748":   "+19452090748",
		"19453003681":   "+19453003681",
		"19452141217":   "+19452141217",
		"18407778097":   "+18407778097",
		"14482074337":   "+14482074337",
		"18406665259":   "+18406665259",
		"19455009160":   "+19455009160",
		"19452381431":   "+19452381431",
		"12793006305":   "+12793006305",
		"15557043340":   "+15557043340",
		"17712015566":   "+17712015566",
		"16452015566":   "+16452015566",
	}

	for orig, expected := range numbers {
		actual, err := FormatNumberForWA(orig)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
}
