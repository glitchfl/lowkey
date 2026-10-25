# lowkey

The valorant tracker that takes 0 performance.

Its a small windows console app that shows whos in your lobby: their rank, recent form, whos partied up and whether u've played with them before. It does all its lookups during agent select and the loading screen then stays quiet for the rest of the match.

## Running it

Download `lowkey.exe` from the [latest release](https://github.com/glitchfl/lowkey/releases/latest), put it in its own folder and double-click it. It isn't signed, so Windows will probably warn you first. Click "More info", then "Run anyway".

Leave it open all day. It idles at about 0% CPU. Settings live in `lowkey.toml` next to the exe, and edits apply without a restart. If something seems off, run `.\lowkey.exe doctor` in PowerShell from that folder.

## What it won't do

- Look up players in streamer mode. They just show as "hidden".
- Call Riot's name service, touch agent select or show which side a team starts on.
- Use the internet while rounds are being played.
- Touch the game. No injection, memory reading or overlay. It runs at low priority and stays under 100 MB of RAM.

## Building

Go 1.26. `make test` runs the tests against fakeriot, a fake Riot Client and servers, so tests never talk to Riot. `make windows` builds the exe, and pushing a `v*` tag publishes a release.
