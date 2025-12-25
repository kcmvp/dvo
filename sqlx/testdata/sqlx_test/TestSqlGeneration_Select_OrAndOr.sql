-- Expected SQL for TestSqlGeneration_Select_OrAndOr
SELECT orders.id AS orders__id,
       orders.account_id AS orders__account_id,
       orders.amount AS orders__amount,
       orders.created_at AS orders__created_at,
       orders.updated_at AS orders__updated_at,
       orders.created_by AS orders__created_by,
       orders.updated_by AS orders__updated_by
FROM orders
WHERE ((orders.amount = ? OR orders.id = ?) AND (orders.account_id > ? OR orders.amount < ?))
