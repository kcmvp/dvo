-- Expected SQL for TestSqlGeneration_Delete_InNonEmpty
DELETE FROM orders
WHERE orders.id IN (?,?,?)

