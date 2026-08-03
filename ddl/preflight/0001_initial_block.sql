INSERT INTO "blocks"
("blockhash", "parenthash", "number")
VALUES
(
    '0x0',
    NULL,
    0
)
on conflict do nothing;
