package entity

// Entity defines the contract for database-aware models.
type Entity interface {
	Table() string
}
