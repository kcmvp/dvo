SELECT
  accounts.ID AS accounts__ID,
  accounts.Email AS accounts__Email,
  accounts.Nickname AS accounts__Nickname,
  accounts.Balance AS accounts__Balance,
  accounts.CreatedAt AS accounts__CreatedAt,
  accounts.UpdatedAt AS accounts__UpdatedAt,
  accounts.CreatedBy AS accounts__CreatedBy,
  accounts.UpdatedBy AS accounts__UpdatedBy
FROM accounts
WHERE accounts.Email LIKE ?
