-- Where to tell a submitter about their review outcome.
--
-- Deliberately a column on plugins rather than a field on an account: the
-- registry authenticates with GitHub and never learns an address otherwise,
-- and harvesting one from the OAuth profile without asking would collect
-- personal data the submitter did not offer. This is filled in only when the
-- submitter types it into the submission form, and it is never returned by any
-- public endpoint.
ALTER TABLE plugins
  ADD COLUMN notify_email TEXT;

COMMENT ON COLUMN plugins.notify_email IS
  'Opt-in contact address for review notifications. Personal data: never expose it through the API, and clear it when the account is deleted.';
