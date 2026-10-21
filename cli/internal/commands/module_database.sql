-- Provisions one module's isolated database role and schema (rule 53). Idempotent; the caller sets module_name and module_password.
\set ON_ERROR_STOP on
SET client_min_messages = warning;
SELECT format('CREATE ROLE %I LOGIN', :'module_name')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'module_name') \gexec
SELECT format('ALTER ROLE %I LOGIN PASSWORD %L', :'module_name', :'module_password') \gexec
CREATE SCHEMA IF NOT EXISTS :"module_name" AUTHORIZATION :"module_name";
REVOKE ALL ON SCHEMA public FROM :"module_name";
