package models

import "time"

// User model.
type User struct {
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExternalID   string `gorm:"column:external_id; type:varchar(255); not null; unique"`
	UserPhotoURL string `gorm:"column:user_photo_url; type:varchar(255)" binding:"max=255"`
	UserPhotoID  string `gorm:"column:user_photo_id; type:varchar(100)" binding:"max=100"`
	ID           int    `gorm:"primary_key"`
}

// TableName will return table name for User
// It will not work if User is not embedded, but mapped as another type
//
//	type MyUser User // will not work
//
// but
//
//	type MyUser struct { // will work
//	    User
//	}
func (User) TableName() string {
	return "mg_user"
}
