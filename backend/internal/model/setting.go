package model

// Setting stores site-wide configuration as key-value pairs.
type Setting struct {
	Key   string `json:"key"   gorm:"primaryKey;type:varchar(100)"`
	Value string `json:"value" gorm:"type:text"`
}
