SELECT
  accounts.ID AS accounts__ID,
  accounts.Email AS accounts__Email,
  profiles.ID AS profiles__ID,
  profiles.AccountID AS profiles__AccountID
FROM accounts
INNER JOIN profiles ON (accounts.ID = profiles.AccountID)
WHERE accounts.ID = ?
