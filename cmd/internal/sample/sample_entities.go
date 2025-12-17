package sample

import "time"

// Account represents a user account in the system.
type Account struct {
	ID        int64  `xql:"pk"`
	Email     string `xql:"unique;index"`
	Nickname  string `xql:"name:nick_name;type:varchar(100);unique;not null;default:'anonymous'"`
	Balance   float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (a Account) Table() string {
	return "accounts"
}

// Order represents a customer order.
type Order struct {
	ID            int64 `xql:"pk"`
	AccountID     int64
	Amount        float64
	internalNotes string
	CreatedAt     time.Time
}

func (o Order) Table() string {
	return "orders"
}
