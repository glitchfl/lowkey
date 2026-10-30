# lowkey

The valorant tracker that takes 0 performance.

Its a small windows console app that shows whos in your lobby: their rank, recent form, whos partied up and whether u've played with them before. It does all its lookups during agent select and the loading screen then stays quiet for the rest of the match.

## Running it

Download `lowkey.exe` from the [latest release](https://github.com/glitchfl/lowkey/releases/latest), put it in its own folder and double-click it. It isn't signed, so Windows will probably warn you first. Click "More info", then "Run anyway".

Leave it open all day. It idles at about 0% CPU. Press Tab to open the settings and change them with the arrow keys. They save straight away to `lowkey.toml` next to the exe, and you can edit that file by hand too, no restart needed. If something seems off, run `.\lowkey.exe doctor` in PowerShell from that folder.

## What it won't do

- Show the names of players in streamer mode. They show as "hidden" with their agent and stats, and turning off `hidden_player_stats` leaves just the agent.
- Call Riot's name service or touch agent select.
- Use the internet while rounds are being played.
- Touch the game. No injection, memory reading or overlay. It runs at low priority and stays under 100 MB of RAM.

## Building

Go 1.26. `make test` runs the tests against fakeriot, a fake Riot Client and servers, so tests never talk to Riot. `make windows` builds the exe, and pushing a `v*` tag publishes a release.
