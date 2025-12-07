package entity

import (
	"fmt"
	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/xql"
	"testing"
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

var _ Entity = Product{}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (u User) Table() string {
	return "t_user"
}

var _ Entity = User{}

// --- Generated Field Definitions (simulated 'dog' output) ---
// The generated code would now call the top-level POField factory function.
var (
	ProductName  = POField[Product, string]("Name")
	ProductPrice = POField[Product, float64]("Price")
	UserName     = POField[User, string]("Name")
)

func TestSchemaComposition(t *testing.T) {
	// This demonstrates creating a schema for the view layer using a simple dvo.Field.
	// Note: We might need to re-introduce a ViewField factory if we want to keep this separation.
	// For now, we assume the view layer can also use dvo.Field directly.
	vo := dvo.WithFields(
		ProductName,
		ProductPrice,
	)

	fmt.Println(vo)

	// This demonstrates creating a type-safe, persistence-aware schema.
	// It uses the NewSchema factory, which returns a provider strictly typed to the entity.
	po := xql.NewSchema[Product](
		ProductName,
		ProductPrice,
	)

	fmt.Println(po)

}
