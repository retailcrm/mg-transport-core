package core

import (
	"github.com/pkg/errors"
)

// ErrorCollector can be used to group several error calls into one call.
// It is mostly useful in GORM migrations, where you should return only one errors, but several can occur.
// Error messages are composed into one message. For example:
// 		err := core.ErrorCollector(
// 			errors.New("first"),
// 			errors.New("second")
// 		)
//
// 		// Will output `first < second`
// 		fmt.Println(err.Error())
// Example with GORM migration, returns one migration error with all error messages:
// 		return core.ErrorCollector(
// 			db.CreateTable(models.Account{}, models.Connection{}).Error,
// 			db.Table("account").AddUniqueIndex("account_key", "channel").Error,
// 		)
func ErrorCollector(errorsList ...error) error {
	var errorMsg string

	for _, errItem := range errorsList {
		if errItem == nil {
			continue
		}

		errorMsg += "< " + errItem.Error() + " "
	}

	if errorMsg != "" {
		return errors.New(errorMsg[2 : len(errorMsg)-1])
	}

	return nil
}
