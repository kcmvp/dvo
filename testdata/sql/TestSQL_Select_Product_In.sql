SELECT
  products.ID AS products__ID,
  products.SKU AS products__SKU,
  products.Name AS products__Name,
  products.Price AS products__Price,
  products.CreatedAt AS products__CreatedAt,
  products.UpdatedAt AS products__UpdatedAt,
  products.CreatedBy AS products__CreatedBy,
  products.UpdatedBy AS products__UpdatedBy
FROM products
WHERE products.ID IN (?,?)
