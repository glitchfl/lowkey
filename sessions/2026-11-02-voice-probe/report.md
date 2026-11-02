# lowkey session report · 2026-11-02 · voice-chat probe

- operator: Claude Code (Opus 5.5) on the Windows PC, nobody at the keyboard. Times are local (UTC+02:00)
- question: does Valorant's voice chat show up in the Riot Client's local voice-chat API, and what do a session and its participants look like?
- what was run: 8 GETs against the local Riot Client API (`https://127.0.0.1:<port>`, Basic auth from the lockfile), one at a time and at least 2 s apart, 15:09 to 15:12. No POST/PUT/DELETE, no WebSocket, nothing remote, and no transcription, report, party or name-service endpoints
- game state: Valorant open in the main menu, not in a party voice channel or a match
- no recordings: lowkey wasn't run. The raw voice responses were deleted after their shapes were taken, and they held no player IDs or names anyway
- files: `local-api-help-full.json` is the full `/help?format=Full` dump, which is static client docs with no personal data. `voice-chat-defs.json` is a verbatim extract of the functions and types below. `extras/scripts/` holds the single-GET fetcher and the shape printer that masks UUIDs and tokens

## Answer

**Nothing for Valorant in the menu.** The voice plugin is up (`status` is `ready`, `v2/state` is `connected: true`), but no product is registered:

- `v1/active-product` returns 204 with an empty body
- `v2/sessions` is an empty list
- `v3` and `v4` `sessions/valorant` both return 500 with "couldn't find any product"

So there's no session to show a voice id format for. The menu can't settle whether a match would register one. That needs the follow-up at the end.

## Requests

| path | status | shape |
|---|---|---|
| `/help?format=Full` | 200 | `{events: list[64], functions: list[1247], types: list[3770]}`, 2.6 MB |
| `/voice-chat/v1/status` | 200 | string, `"ready"` (`VoiceChatPluginStatus`) |
| `/voice-chat/v1/active-product` | **204** | empty body |
| `/voice-chat/v2/sessions` | 200 | `list[0]` |
| `/voice-chat/v3/sessions/valorant` | **500** | `{errorCode: "RPC_ERROR", httpStatus: 500, implementationDetails: {}, message: str}` |
| `/voice-chat/v4/sessions/valorant` | **500** | byte-identical to v3 |
| `/voice-chat/v2/settings` | 200 | `{currentCaptureDeviceHandle: str, currentRenderDeviceHandle: str, localMicMuted: bool, loopbackEnabled: bool, micLevel: int, noiseSuppressionAlgorithm: str, noiseSuppressionLevel: str, speakerLevel: int, vadAuto: bool, vadHangoverTime: int, vadSensitivity: int, voiceFontId: str ""}` |
| `/voice-chat/v2/state` | 200 | `{connected: bool}`, `true` |

The v3/v4 message: `We couldn't find any product for session '<rc-session-id>'. You might want to try retrying soon.` The id is a 22-character base64url-style string, probably the Riot Client's own session rather than a voice one. With the empty active product, this looks like Valorant never registered with the client's voice plugin. It may run its own voice in-game, but that's a guess until the match probe.

`/help` marks `v2/sessions`, `v2/settings` and `v2/state` as deleted (2020–2021), but they still answer 200.

## Paths

`?format=Full` gives full definitions (arguments, return types, type fields, enum values) but **no URL path field**. Paths follow from the function names (kebab-case, like `push-to-talk` in the help text), and several appear literally in the help.

| function | path | arguments → returns |
|---|---|---|
| GetVoiceChatV1Status | `GET /voice-chat/v1/status` | none → `VoiceChatPluginStatus` |
| GetVoiceChatV1ActiveProduct | `GET /voice-chat/v1/active-product` | none → `string` |
| GetVoiceChatV2Sessions | `GET /voice-chat/v2/sessions` | none → `vector<VoiceChatSessionResource>`; help: "Endpoint deleted March 2021." |
| GetVoiceChatV4SessionsByProductName | `GET /voice-chat/v4/sessions/{productName}` (literal in help) | `productName: string` → `vector<VoiceChatSessionResource>` |
| GetVoiceChatV4SessionsByProductNameById | `GET /voice-chat/v4/sessions/{productName}/{id}` (literal in help) | `productName, id: string` → `VoiceChatSessionResource` |
| GetVoiceChatV4SessionsByProductNameBySessionIdParticipantsByParticipantId | `GET /voice-chat/v4/sessions/{productName}/{sessionId}/participants/{participantId}` (from the name; matches the literal v5 `.../participants/{participantId}/report`) | `productName, sessionId, participantId: string` → `VoiceChatParticipantResource` |

All three v3 session GETs say "deprecated and will be deleted November 2022. Please use /voice-chat/v4/sessions/{productName}".

## Types

These are complete. In every field, `description` is `""`, `offset` is 0 and `optional` is false; each type also has `description ""`, `nameSpace ""`, `size 0` and `tags []`. The raw JSON is in `voice-chat-defs.json`.

```
VoiceChatSessionResource { id: string, isFocused: bool, isMuted: bool, isRestricted: bool,
  isTranscriptionEnabled: bool, isTransmitEnabled: bool, isVoiceFontEnabled: bool,
  participants: vector<VoiceChatParticipantResource>, product: string, provider: string,
  status: VoiceChatSessionStatus, terminationReason: VoiceChatSessionTerminationReason, volume: uint32 }
VoiceChatParticipantResource { energy: uint32, id: string, isInteractionAllowed: bool, isMuted: bool,
  isRestricted: bool, isSpeaking: bool, name: string, platform: string, speakingTimestamp: uint32,
  uri: string, volume: uint32 }
VoiceChatSessionStatus (enum) active=0, onHold=1, initializing=2, connecting=3, connected=4,
  finalizing=5, disconnecting=6, disconnected=7, reconnecting=8
VoiceChatProductSession { patchlineFullName: string, patchlineId: string, productId: string, version: string }
  (no function or type references it)
VoiceChatSessionTerminationReason (enum) Requested=0, AloneTimeout=1, AuthenticationEnded=2,
  UnexpectedProviderRemoval=3, Unknown=4
VoiceChatUpdateParticipantResource { isMuted: bool, volume: uint32 }   (body of the participant PUT)
VoiceChatPluginStatus (enum) unknown=0, awaitingAuth=1, permissionUnknown=2, permissionDenied=3,
  networkUnavailable=4, noProviders=5, ready=6
```

The session PUT help explains three session fields: "volume: Controls the output volume for a room. Set to 0 to mute. isFocused: Toggles if a session gains volume priority, which dampens the volume of the other sessions. isTransmitEnabled: Toggles audio input for a session."

## What a participant's fields could mean

| field | type | reading |
|---|---|---|
| `energy` | uint32 | audio level. A threshold on it could stand in for **speaking** |
| `id` | string | probably the player's PUUID (guess) |
| `isInteractionAllowed` | bool | whether you're allowed to mute or report them (guess) |
| `isMuted` | bool | **muted by me** |
| `isRestricted` | bool | probably comms-restricted by a penalty (guess) |
| `isSpeaking` | bool | **speaking** |
| `name` | string | probably a display name or Riot ID, so scrub it like any name |
| `platform` | string | |
| `speakingTimestamp` | uint32 | when they last spoke |
| `uri` | string | probably the voice provider's address for them (guess) |
| `volume` | uint32 | the volume you set for them |

- **Muted by me:** `isMuted` is almost certainly this. The only participant PUT takes `{isMuted, volume}`, both required, which is something you set on others. The block-list help also says blocked users "will be muted".
- **Muted by themselves:** no field exposes this. Your own mic state is on the session (`isMuted`, `isTransmitEnabled`), not on other participants.
- **Connected:** participants have no flag for this; being in the list is the signal. The session has `status`.

## Skipped and failed

- Step 3, a single session and participant by id, was skipped because nothing listed a session, and no ids were guessed
- the v3 and v4 `sessions/valorant` requests failed with 500 (above)
- two local mistakes sent no request:
  - `python -I` hid the user-installed `requests` package
  - Git Bash turned `/voice-chat/...` into a Windows path, and the fetcher's path check stopped it

## Follow-up

Run the same `v1/active-product` and `v4/sessions/valorant` GETs twice: once in a match with team voice on, and once in a party with party voice on. If a `valorant` session appears, GET it by id and one participant. Expect team ids like `<matchId>-tm1`.
