-- ============================================================================
-- Supabase migration: wire the app's "public.*" data layer to Supabase Auth
-- ----------------------------------------------------------------------------
-- Apply this against a Supabase Postgres project AFTER the app schema exists.
-- The app schema itself (users, health_records, ... + the `auth_uuid` column)
-- is bootstrapped automatically by the Go backend on connect (see
-- internal/database/schema.sql), so this file only adds the Supabase-managed
-- pieces that live in the `auth` schema / RLS:
--
--   1. auto-provision an app `public.users` row for every new Supabase user
--      (from auth.users), reading role/provider fields from user metadata
--   2. enable RLS on the app tables + a deny-by-default policy so a client
--      holding only the anon/publishable key cannot read every row
--
-- How to run (idempotent — safe to re-run):
--   psql "$DIRECT_URL" -f supabase_migrate.sql
-- (DIRECT_URL = the session-mode pooler DSN, port 5432; Supabase DDL is
--  blocked on the transaction-mode pooler port 6543.)
--
-- Verify afterwards:
--   psql "$DIRECT_URL" -c "SELECT tgname FROM pg_trigger
--      WHERE tgrelid='auth.users'::regclass AND NOT tgisinternal;"
--   -- expect: on_auth_user_created
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 0. Prerequisites the Go bootstrap provides (no-ops if already applied).
--    Kept here so this file is self-contained when the Go backend has not
--    yet connected to the DB.
-- ----------------------------------------------------------------------------
ALTER TABLE public.users ADD COLUMN IF NOT EXISTS auth_uuid UUID;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_auth_uuid ON public.users (auth_uuid);

-- ----------------------------------------------------------------------------
-- 1. Trigger function: provision + sync the app user row from a new Supabase
--    identity. Runs as a SECURITY DEFINER trigger on auth.users, so it can
--    write to public.users even though the auth insert is issued by GoTrue.
--
--    Role + provider fields travel in raw_user_meta_data (set by the frontend
--    at signup via options.data). Falls back to sensible defaults.
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER AS
$$
DECLARE
  m jsonb := coalesce(new.raw_user_meta_data, '{}'::jsonb);
  p_role text;
BEGIN
  p_role := coalesce(m->>'role', 'patient');
  IF p_role NOT IN ('patient','doctor','provider') THEN
    p_role := 'patient';
  END IF;

  INSERT INTO public.users (
    username, email, password, first_name, last_name, user_type, is_approved,
    is_email_verified, city, profile_image, auth_uuid
  ) VALUES (
    coalesce(m->>'username', split_part(coalesce(new.email,'user@x'), '@', 1)),
    lower(coalesce(new.email, m->>'email', 'unknown')),
    '$2y$10$0000000000000000000000000000000000000000000000000000', -- Supabase owns the real password; placeholder not used for login
    coalesce(m->>'first_name', split_part(coalesce(new.email,'n'), '@', 1)),
    coalesce(m->>'last_name', ''),
    CASE WHEN p_role IN ('doctor','provider') THEN 'provider' ELSE 'patient' END,
    CASE WHEN p_role = 'patient' THEN 1 ELSE 0 END,
    1,
    coalesce(m->>'city', coalesce(m->>'state','')),
    coalesce(m->>'picture',''),
    new.id
  )
  ON CONFLICT (auth_uuid) DO UPDATE SET
    user_type = EXCLUDED.user_type,
    is_email_verified = 1,
    updated_at = NOW();

  RETURN new;
END;
$$;

DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW EXECUTE FUNCTION public.handle_new_user();

-- ----------------------------------------------------------------------------
-- 2. Row Level Security.
--
-- The Go backend connects as the `postgres` table OWNER. In Postgres the
-- owner bypasses RLS by default, so enabling RLS does NOT break the backend
-- while still protecting a client that only has the anon/publishable key.
--
-- Enable RLS on the app tables and add a deny-by-default policy for anon +
-- authenticated (raw PostgREST access). The Go backend, as owner, still
-- reads/writes normally.
-- ----------------------------------------------------------------------------
DO $$
BEGIN
  EXECUTE 'ALTER TABLE public.users ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.health_records ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.medicines ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.prescriptions ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.mental_health_logs ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.lifestyle_logs ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.insurance_policies ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.appointments ENABLE ROW LEVEL SECURITY';
  EXECUTE 'ALTER TABLE public.service_requests ENABLE ROW LEVEL SECURITY';
END
$$;

-- Deny-by-default on the identity table so the anon key cannot enumerate
-- users. (Other tables inherit the same effect: no permissive policy = deny.)
DROP POLICY IF EXISTS users_deny_anon ON public.users;
CREATE POLICY users_deny_anon ON public.users
  FOR ALL TO anon, authenticated
  USING (false) WITH CHECK (false);
