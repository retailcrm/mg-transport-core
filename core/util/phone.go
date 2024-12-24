package util

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	phoneiso3166 "github.com/onlinecity/go-phone-iso3166"
	pn "github.com/ttacon/libphonenumber"
)

const (
	MinPhoneSymbolCount = 5
	CountryPhoneCodeDE  = 49
	CountryPhoneCodeAG  = 54
	CountryPhoneCodeMX  = 52
	CountryPhoneCodeUS  = "1443"
	CountryPhoneCodePS  = 970
	CountryPhoneCodeUZ  = 998
	PalestineRegion     = "PS"
	BangladeshRegion    = "BD"
)

var (
	ErrPhoneTooShort          = errors.New("phone is too short - must be at least 5 symbols")
	ErrCannotDetermineCountry = errors.New("cannot determine phone country code")
	ErrCannotParsePhone       = errors.New("cannot parse phone number")

	TrimmedPhoneRegexp = regexp.MustCompile(`\D+`)
	UndefinedUSCodes   = []string{"1445", "1945", "1840", "1448", "1279", "1839"}
)

// FormatNumberForWA forms in the format according to the rules https://faq.whatsapp.com/1294841057948784
func FormatNumberForWA(number string) (string, error) {
	parsedPhone, err := ParsePhone(number)

	if err != nil {
		return "", err
	}

	var formattedPhoneNumber string
	switch parsedPhone.GetCountryCode() {
	case CountryPhoneCodeAG:
		formattedPhoneNumber = Add9AGIFNeed(parsedPhone)
	default:
		formattedPhoneNumber = pn.Format(parsedPhone, pn.E164)
	}

	return formattedPhoneNumber, nil
}

// ParsePhone this function parses the number as a string
// For Mexican numbers `1` is always added to the national number because it is always removed during parsing.
// Attention when formatted in libphonenumber.INTERNATIONAL 1 will not be after the country code, even though
// it is in the national number.
// But for Argentine numbers there is no automatic addition 9 to the country code.
func ParsePhone(phoneNumber string) (*pn.PhoneNumber, error) {
	trimmedPhone := TrimmedPhoneRegexp.ReplaceAllString(phoneNumber, "")
	if len(trimmedPhone) < MinPhoneSymbolCount {
		return nil, ErrPhoneTooShort
	}

	countryCode := getCountryCode(trimmedPhone)
	if countryCode == "" {
		return nil, ErrCannotDetermineCountry
	}

	parsedPhone, err := pn.Parse(trimmedPhone, countryCode)
	if err != nil {
		return nil, ErrCannotParsePhone
	}

	if CountryPhoneCodeDE == parsedPhone.GetCountryCode() {
		number, err := getGermanNationalNumber(trimmedPhone, parsedPhone)
		if err != nil {
			return nil, err
		}

		parsedPhone.NationalNumber = &number
	}

	if CountryPhoneCodeUZ == parsedPhone.GetCountryCode() {
		number, err := getUzbekistanNationalNumber(trimmedPhone, parsedPhone)
		if err != nil {
			return nil, err
		}

		parsedPhone.NationalNumber = &number
	}

	if IsMexicoNumber(parsedPhone) {
		number, err := getMexicanNationalNumber(parsedPhone)
		if err != nil {
			return nil, err
		}

		parsedPhone.NationalNumber = &number
	}

	return parsedPhone, err
}

func IsRussianNumberWith8Prefix(phone string) bool {
	return strings.HasPrefix(phone, "8") && len(phone) == 11 && phoneiso3166.E164.LookupString("7"+phone[1:]) == "RU"
}

func IsMexicoNumber(parsed *pn.PhoneNumber) bool {
	return parsed.GetCountryCode() == CountryPhoneCodeMX
}

func IsUSNumber(phone string) bool {
	return slices.Contains(UndefinedUSCodes, phone[:4]) &&
		phoneiso3166.E164.LookupString(CountryPhoneCodeUS+phone[4:]) == "US"
}

func IsPLNumber(phone string) bool {
	num, err := pn.Parse(phone, "PS")
	return err == nil && num.GetCountryCode() == CountryPhoneCodePS && fmt.Sprintf("%d", CountryPhoneCodePS) == phone[0:3]
}

func Remove9AGIfNeed(parsedPhone *pn.PhoneNumber) string {
	formattedPhone := pn.Format(parsedPhone, pn.E164)
	numberWOCountry := fmt.Sprintf("%d", parsedPhone.GetNationalNumber())

	if len(numberWOCountry) == 11 && string(numberWOCountry[0]) == "9" {
		formattedPhone = fmt.Sprintf("+%d%s", CountryPhoneCodeAG, numberWOCountry[1:])
	}

	return formattedPhone
}

func Add9AGIFNeed(parsedPhone *pn.PhoneNumber) string {
	formattedPhone := pn.Format(parsedPhone, pn.E164)
	numberWOCountry := fmt.Sprintf("%d", parsedPhone.GetNationalNumber())

	if len(numberWOCountry) == 10 { // nolint:mnd
		formattedPhone = fmt.Sprintf("+%d%s", CountryPhoneCodeAG, "9"+numberWOCountry)
	}

	return formattedPhone
}

// getGermanNationalNumber some German numbers may not be parsed correctly.
// For example, for 491736276098 libphonenumber.PhoneNumber.NationalNumber
// will contain the country code(49). This function fix it and return correct libphonenumber.PhoneNumber.
func getGermanNationalNumber(phone string, parsedPhone *pn.PhoneNumber) (uint64, error) {
	result := parsedPhone.GetNationalNumber()

	if len(fmt.Sprintf("%d", parsedPhone.GetNationalNumber())) == len(phone) {
		deduplicateCountryNumber := fmt.Sprintf("%d", parsedPhone.GetNationalNumber())[2:]

		number, err := strconv.Atoi(deduplicateCountryNumber)
		if err != nil {
			return 0, err
		}

		result = uint64(number) //nolint:gosec
	}

	return result, nil
}

// For UZ numbers where 8 is deleted after the country code.
func getUzbekistanNationalNumber(phone string, parsedPhone *pn.PhoneNumber) (uint64, error) {
	result := parsedPhone.GetNationalNumber()
	numberWithEight := fmt.Sprintf("8%d", parsedPhone.GetNationalNumber())

	if len(fmt.Sprintf("%d%s", parsedPhone.GetCountryCode(), numberWithEight)) == len(phone) {
		number, err := strconv.Atoi(numberWithEight)
		if err != nil {
			return 0, err
		}

		result = uint64(number) //nolint:gosec
	}

	return result, nil
}

func getMexicanNationalNumber(parsedPhone *pn.PhoneNumber) (uint64, error) {
	phoneWithDigit := fmt.Sprintf("1%d", parsedPhone.GetNationalNumber())

	num, err := strconv.Atoi(phoneWithDigit)

	if err != nil {
		return 0, err
	}

	return uint64(num), nil //nolint:gosec
}

func getCountryCode(phone string) string {
	countryCode := phoneiso3166.E164.LookupString(phone)

	if countryCode == "" {
		if IsRussianNumberWith8Prefix(phone) {
			countryCode = phoneiso3166.E164.LookupString("7" + phone[1:])
		}

		if IsUSNumber(phone) {
			countryCode = phoneiso3166.E164.LookupString(CountryPhoneCodeUS + phone[4:])
		}

		if IsPLNumber(phone) {
			countryCode = PalestineRegion
		}
	}

	// For russian numbers as 8800xxxxxxx
	if strings.EqualFold(BangladeshRegion, countryCode) && IsRussianNumberWith8Prefix(phone) {
		countryCode = phoneiso3166.E164.LookupString("7" + phone[1:])
	}

	return countryCode
}
