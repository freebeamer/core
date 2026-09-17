# FreeBeamer naming and compatibility

The project was renamed from FreeHorse to **FreeBeamer** before its initial
GitHub publication. The organization is <https://github.com/freebeamer>.

Applications, Go module/import paths, CLI commands, Flutter package names,
and project branding now use FreeBeamer. The source workspace directory and
existing development-server remote are not renamed by this change.

## Existing data

The following identifiers intentionally retain their original spelling:

- `.mapdef` format `freehorse.mapdef` and its original JSON Schema `$id`;
- conversion languages `freehorse-expression-v1` and `freehorse-lookup-table-v1`;
- profile format `freehorse-vehicle-profiles`;
- fingerprint namespaces `freehorse-definition-v1`, `freehorse-profile-v1`,
  and `freehorse-session-v1`;
- existing opaque relay API keys, including the `fh_live_` prefix.

These are versioned contracts, not display branding. Keeping them stable means
existing definitions, conversion formulas, profiles, and identity fingerprints
continue to mean the same thing. Renaming them requires a separately versioned
format migration.

## Development-server migration

No running service or database is changed by the source rename. Before switching
an existing deployment to the new binaries:

1. Rename configuration variables to `FREEBEAMER_RELAY_ADMIN_TOKEN` and
   `FREEBEAMER_RELAY_API_KEY`; retain the actual secret values.
2. Keep using the existing SQLite database explicitly with `--db`, or migrate
   a backed-up database to the new default `freebeamer-relay.sqlite`. Preserve
   the Docker volume; an empty new database will not contain existing clients.
3. Rebuild the mobile platform shells with `com.freebeamer` and register the
   `freebeamer://connect` scheme. Existing installed apps are not updated by
   editing Dart source. Generate new provisioning links for renamed clients.
4. If needed, copy the old desktop `freehorse/vehicle-profiles-v1.json` into
   `freebeamer/vehicle-profiles-v1.json` under the user configuration directory.
   Its contents remain compatible. New logs use `FreeBeamer/freebeamer.log`.

Historical screenshots can still show the earlier name; repository banners
and current application text use FreeBeamer.
