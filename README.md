# lowkey

The valorant tracker that takes 0 performance.

Its a small windows console app that shows whos in your lobby: their rank, recent form, whos partied up and whether u've played with them before. It does all its lookups during agent select and the loading screen then stays quiet for the rest of the match.

## Running it

Download `lowkey.exe` from the [latest release](https://github.com/glitchfl/lowkey/releases/latest), put it in its own folder and double-click it. It isn't signed, so Windows will probably warn you first. Click "More info", then "Run anyway".

Leave it open all day. It idles at about 0% CPU. Press Tab to open the settings and change them with the arrow keys. The settings are grouped into table, players, match, general and agent select. They save straight away to `lowkey.toml` next to the exe, which has the same groups as `[table]`, `[players]`, `[match]`, `[general]` and `[agent_select]`. You can edit that file by hand too, no restart needed. A `lowkey.toml` from an older version gets moved into the groups the first time a newer lowkey starts, and the old one is kept as `lowkey.toml.bak`. If something seems off, run `.\lowkey.exe doctor` in PowerShell from that folder.

To see more about one player, turn on player details in the settings, then pick them with the up and down arrows and press Enter. You get their rank in every act, their record in each mode this act, their recent matches and agents, and the skin and buddy on every gun. Left and right switch players and Esc goes back. It only shows what lowkey already downloaded, so it works mid-match too and never sends anything.

If you like locking your agent fast, there are agent dots. They need Valorant in Windowed or Windowed Fullscreen. Start a custom game, and in its agent select switch to lowkey, press Tab, open agent dots and pick an agent with left and right. Press Enter, switch back and rest the cursor on that agent. After 5 seconds lowkey saves the spot, turns the dots on and shows that one there for a moment so you can check it. You can set up 3 agents this way. From then on, when a match is found, each one gets a small dot in its colors with its name above, so your cursor can be on your pick when agent select opens. They all go as soon as you pick an agent, after 15 seconds (dot time changes that) or when agent select ends, and a teammate locking one of your agents takes that one away. You still click your agent and Lock In yourself.

## What it won't do

- Show the names of players in streamer mode. They show as "hidden" with their agent and stats, and turning off `hidden_player_stats` leaves just the agent. It only asks Riot's name service about everyone else.
- Touch agent select. It never picks or locks an agent.
- Use the internet while rounds are being played.
- Touch the game. No injection or memory reading. The only thing it ever draws outside its window is the agent dots, if you turn them on: up to 3 small named dots for a few seconds after a match is found, never during rounds. It runs at low priority and stays under 100 MB of RAM.

## Building

Go 1.26. `make test` runs the tests against fakeriot, a fake Riot Client and servers, so tests never talk to Riot. `make windows` builds the exe, and pushing a `v*` tag publishes a release.
