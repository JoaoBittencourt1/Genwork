-- Deletes one module's role, schema, and data when it is uninstalled (rule 56; documented exception to rule 22).
-- Idempotent; the caller sets module_name.
\set ON_ERROR_STOP on
SET client_min_messages = warning;
SELECT format('DROP OWNED BY %I CASCADE', :'module_name')
WHERE EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'module_name') \gexec
DROP SCHEMA IF EXISTS :"module_name" CASCADE;
DROP ROLE IF EXISTS :"module_name";
