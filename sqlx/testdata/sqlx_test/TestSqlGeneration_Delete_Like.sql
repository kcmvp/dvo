-- Expected SQL for TestSqlGeneration_Delete_Like
DELETE FROM orders
WHERE orders.created_by LIKE ?

