-- Down: Postgres não permite remover valores de enum sem recriar o tipo.
-- O rollback da 000003 é um no-op proposital (valores extras são inofensivos).
-- Se for necessário recriar: DROP TYPE ... CASCADE e re-aplicar a 000001.
SELECT 1;
