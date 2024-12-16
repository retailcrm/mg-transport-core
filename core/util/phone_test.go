package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParsePhone(t *testing.T) {
	t.Run("russian numers", func(t *testing.T) {
		n := "+88002541213"
		pn, err := ParsePhone(n)
		assert.Equal(t, uint64(8002541213), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

		n = "+78002541213"
		pn, err = ParsePhone(n)
		assert.NoError(t, err)
		assert.NotNil(t, pn)
		assert.Equal(t, uint64(8002541213), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())

		n = "89521548787"
		pn, err = ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(9521548787), pn.GetNationalNumber())
		assert.Equal(t, int32(7), pn.GetCountryCode())
	})

	t.Run("german numbers", func(t *testing.T) {
		n := "491736276098"
		pn, err := ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1736276098), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeDE), pn.GetCountryCode())

		n = "4915229457499"
		pn, err = ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(15229457499), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeDE), pn.GetCountryCode())
	})

	t.Run("mexican number", func(t *testing.T) {
		n := "5219982418333"
		pn, err := ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(9982418333), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeMX), pn.GetCountryCode())
	})

	t.Run("palestine number", func(t *testing.T) {
		n := "970567800663"
		pn, err := ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(567800663), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodePS), pn.GetCountryCode())
	})

	t.Run("argentine number", func(t *testing.T) {
		n := "5491131157821"
		pn, err := ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(91131157821), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeAG), pn.GetCountryCode())
	})

	t.Run("uzbekistan number", func(t *testing.T) {
		n := "998882207724"
		pn, err := ParsePhone(n)
		assert.NoError(t, err)
		assert.Equal(t, uint64(882207724), pn.GetNationalNumber())
		assert.Equal(t, int32(CountryPhoneCodeUZ), pn.GetCountryCode())
	})
}
