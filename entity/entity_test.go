package entity_test

import (
	"fmt"
	"testing"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/entity"
)

// --- Base Entity Definitions ---

type Product struct {
	ID         int     `json:"ID"`
	Name       string  `json:"Name"`
	Price      float64 `json:"Price"`
	CategoryID int     `json:"CategoryID"`
}

func (p Product) Table() string {
	return "t_product"
}

var _ entity.Entity = Product{}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) Table() string {
	return "t_user"
}

var _ entity.Entity = User{}

// --- Generated Field Definitions (simulated 'dog' output) ---
// The generated code would now call the top-level Field factory function.
var (
	ProductName  = entity.Field[Product, string]("Name")
	ProductPrice = entity.Field[Product, float64]("Price")
	UserName     = entity.Field[User, string]("Name")
)

func TestSchemaComposition(t *testing.T) {
	// This demonstrates creating a schema-like value using dvo.WithFields and the
	// entity.Field factory without importing sqlx to avoid import cycles.
	vo := dvo.WithFields(
		ProductName,
		ProductPrice,
	)

	fmt.Println(vo)

	// Simple assertion: ensure the qualified name includes the table prefix.
	if got := ProductName.AsSchemaField().Name(); got != "Name" {
		t.Fatalf("expected field name 'Name', got %q", got)
	}

}
