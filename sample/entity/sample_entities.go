package entity

import "time"

// BaseEntity defines common fields for database entities.
type BaseEntity struct {
	ID        int64 `xql:"pk"`
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

// Account represents a user account in the system.
type Account struct {
	BaseEntity
	Email    string `xql:"unique;index"`
	Nickname string `xql:"name:nick_name;type:varchar(100);unique;not null;default:'anonymous'"`
	Balance  float64
}

func (a Account) Table() string {
	return "accounts"
}

// Order represents a customer order.
type Order struct {
	BaseEntity
	AccountID     int64
	Amount        float64
	internalNotes string
}

func (o Order) Table() string {
	return "orders"
}
