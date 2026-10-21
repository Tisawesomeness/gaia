# Gaia

A Discord bot keeping watch over Orbis. Gaia is a Hytale utility bot with features such as:

- Username <-> UUID conversion
- Check if a username is reserved
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

### Username Reservation

Username reservation lookups require your Hytale account credentials to be set in either `config.json` or in environment variables:

```
GAIA_HYTALE_EMAIL
GAIA_HYTALE_PASSWORD
GAIA_HYTALE_2FA_SECRET
```
