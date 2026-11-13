-- Preserve award/payment history while retiring non-trending challenges.
UPDATE challenges SET active = false WHERE id NOT IN ('tt', 'tut');
