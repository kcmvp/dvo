SELECT
  accounts.ID AS accounts__ID,
  profiles.ID AS profiles__ID
FROM accounts
LEFT JOIN profiles ON ((accounts.ID = profiles.AccountID) AND (profiles.Bio LIKE ?))
