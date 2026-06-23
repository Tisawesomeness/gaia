# Gaia

A Discord bot keeping watch over Orbis. Gaia is a Hytale utility bot with features such as:

- Username <-> UUID conversion
- Skin lookup
- Get notified/pinged for new Hytale versions
- Browse launcher articles
- User and guild install support
- More coming out as the Hytale API develops

Install or invite the bot at https://tis.codes/gaia

## Self-Hosting

Fill out `config.json` with your Discord token, or set the `GAIA_DISCORD_TOKEN` environment variable.

```sh
# Start the Valkey (Redis fork) server
valkey-server ./valkey.conf &

# Start the bot
go run .
```

It will prompt you with an OAuth login flow, like when running a Hytale server.

## Test

```sh
# Start the Valkey test server
valkey-server ./valkey_test.conf &

# Run tests
go test ./...
```
