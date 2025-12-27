SELECT
  accounts.ID AS accounts__ID,
  profiles.ID AS profiles__ID
FROM accounts
INNER JOIN profiles ON (accounts.ID = profiles.AccountID)
WHERE ((accounts.ID = ? AND profiles.Bio LIKE ?) OR (accounts.ID = ? AND profiles.Bio LIKE ?))
