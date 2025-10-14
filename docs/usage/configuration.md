---
title: Configuration
---

# Configuration

The Configuration page lets administrators control key global settings for the Magi instance.

## Access

Only users with the `admin` role can access `/configuration`. The initial user to sign up on a brand new instance automatically becomes `admin`.

## Settings

| Setting | Description |
| ------- | ----------- |
| Allow new user registrations | Toggle whether anonymous visitors can create new accounts. When off, both the Register page and POST attempts to `/register` return an error page. |
| Max users | Hard limit on total user accounts. Set to `0` for unlimited. Once the limit is reached, further registrations are blocked even if registration is enabled. |

These values are stored in the single-row `app_config` table. Updating them via the UI immediately applies the new rules.

## Behavior Summary

1. If Allow Registration = off -> All new registrations are rejected.
2. If Max Users > 0 and current user count >= Max Users -> Registrations are rejected.
3. First ever user created is auto-promoted to admin (unchanged behavior).

## Troubleshooting

If you accidentally lock yourself out by disabling registration before any admin exists (unlikely because the first user becomes admin), you can manually re-enable it by running:

```sql
UPDATE app_config SET allow_registration = 1 WHERE id = 1;
```

Or remove the limit:

```sql
UPDATE app_config SET max_users = 0 WHERE id = 1;
```

## Future Ideas

Potential future expansion could include:

- Read-only maintenance mode
- SMTP / notification settings
- Theme toggles
- Rate limiting controls

Contributions are welcome!# Magi configuration guide

More to come :)
