package models

import "time"

// Connection model.
type Connection struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Key       string    `gorm:"column:api_key; type:varchar(100); not null" json:"apiKey,omitempty" binding:"required,max=100"`                // nolint:lll
	URL       string    `gorm:"column:api_url; type:varchar(255); not null" json:"apiUrl,omitempty" binding:"required,validateCrmURL,max=255"` // nolint:lll
	GateURL   string    `gorm:"column:mg_url; type:varchar(255); not null;" json:"mgUrl,omitempty" binding:"max=255"`
	GateToken string    `gorm:"column:mg_token; type:varchar(100); not null; unique" json:"mgToken,omitempty" binding:"max=100"` // nolint:lll
	ClientID  string    `gorm:"column:client_id; type:varchar(70); not null; unique" json:"clientId,omitempty"`
	Lang      string    `gorm:"column:lang; type:varchar(2)" json:"lang,omitempty" binding:"max=2"`
	PublicURL string    `gorm:"column:public_url; type:varchar(255);" json:"publicUrl,omitempty" binding:"max=255"`
	Accounts  []Account `gorm:"foreignkey:ConnectionID" json:"accounts"`
	ID        int       `gorm:"primary_key" json:"id"`
	Active    bool      `json:"active,omitempty"`
}
