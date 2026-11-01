# Endpoint census Â· 2026-10-27 data discovery session

Built from Valorant's own log, `%LOCALAPPDATA%\VALORANT\Saved\Logs\ShooterGame.log`, without sending any request. Times are the log's, in UTC. Every UUID is `{id}`, every 32-character hex ID (a UUID without dashes, used for agents, matches and sometimes players) is `{hex}`, every numeric ID is `{n}`. Query strings keep their keys only.

Three Swift (`swiftplay`) matches were played: match 1 ended about 17:12:50, match 2 about 17:37:13 and match 3 about 17:56:12 UTC (the log's `Reconcile called with state: InGame` line). Rows first seen during a match are marked with it in the last column.

## Log line formats

Requests the game makes through its platform layer, one line per finished request:
```
[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogPlatformCommon: Platform HTTP Query End. QueryName: [<name>], URL [<METHOD> https://<host>/<path>?<query>], @trace_id:<32 hex> Response Code: [<code>], Seconds Since Query [<seconds>]
```
Requests from the replay data service, logged when created, with no response line:
```
[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogReplayData: Replay Data Service: [<pointer>] Created <METHOD> query <NAME> to url <https://<host>/<path>?<query>
```
Pushes from the Riot messaging service (not requests, listed in the last table):
```
[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogRMSService: Received RMS update. URI: /riot-messaging-service/v1/messages/<service>/<path>
```
The screen the client shows, which dates the tour steps:
```
[YYYY.MM.DD-HH.MM.SS:mmm][frame]LogUINavigationModel: Warning: Current Url: main/hub/<screen>/<subscreen>
```

## This session (current log, game started 16:48 UTC)

| Query name | Method | Host | Path | Query keys | Count | Codes | First seen | First triggered by |
|---|---|---|---|---|---|---|---|---|
| Config_FetchConfig | GET | shared.eu.a.pvp.net | `/v1/config/eu` |  | 78 | 200x78 | 16:48:22 | startup, before the tour |
| Store_GetAgentStorefront | GET | pd.eu.a.pvp.net | `/store/v1/storefronts/agent` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Party_FetchCustomGameConfigs | GET | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/customgameconfigs` |  | 39 | 200x39 | 16:48:24 | startup, before the tour |
| Premier_GetPlayer_V2 | GET | pd.eu.a.pvp.net | `/premier/v2/players/{id}` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Content_FetchContent | GET | shared.eu.a.pvp.net | `/content-service/v3/content` |  | 19 | 200x19 | 16:48:24 | startup, before the tour |
| AccountXP_GetPlayer | GET | pd.eu.a.pvp.net | `/account-xp/v1/players/{id}` |  | 4 | 200x4 | 16:48:24 | startup, before the tour |
| MassRewards_ReconcilePlayer | POST | pd.eu.a.pvp.net | `/mass-rewards/v1/players/{id}/reconcile` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Bloomline_FetchValidQueues | GET | pd.eu.a.pvp.net | `/bloomline/v0/config/valid-queues` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Premier_GetActivePremierSeason | GET | pd.eu.a.pvp.net | `/premier/v1/affinities/eu/premier-seasons/active` |  | 2 | 200x2 | 16:48:24 | startup, before the tour |
| Store_GetAllEntitlements | GET | pd.eu.a.pvp.net | `/store/v1/entitlements/{id}` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| ItemProgressionDefinitionsV2_Fetch | GET | pd.eu.a.pvp.net | `/contract-definitions/v3/item-upgrades` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Premier_GetPremierConferences | GET | pd.eu.a.pvp.net | `/premier/v1/affinities/eu/conferences` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Bloomline_FetchPlayer | GET | pd.eu.a.pvp.net | `/bloomline/v0/players/{id}` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| MMR_FetchPlayer, MMR_FetchPlayer_WithPrestigeBackfill | GET | pd.eu.a.pvp.net | `/mmr/v1/players/{id}` | `backfill` | 5 | 200x5 | 16:48:24 | startup, before the tour |
| Store_GetWallet | GET | pd.eu.a.pvp.net | `/store/v1/wallet/{id}` |  | 4 | 200x4 | 16:48:24 | startup, before the tour |
| DailyRewards_RENEW | POST | pd.eu.a.pvp.net | `/daily-ticket/v1/{id}/renew` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| GoldStar_GetGoldStars | GET | pd.eu.a.pvp.net | `/goldstars/v1/goldstars` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| DisplayNameService_UpdatePlayer | POST | pd.eu.a.pvp.net | `/name-service/v3/players` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| playerLoadoutUpdate | GET | pd.eu.a.pvp.net | `/personalization/v3/players/{id}/playerloadout` |  | 2 | 200x2 | 16:48:24 | startup, before the tour |
| DailyRewards_GET_QUEUE_CONFIG | GET | pd.eu.a.pvp.net | `/daily-ticket/v1/queue-config` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Experimentation_GroupAssignments | POST | pd.eu.a.pvp.net | `/experimentation/v2/group-assignments/{id}` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Favorites_GetFavorites | GET | pd.eu.a.pvp.net | `/favorites/v1/players/{id}/favorites` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Session_Connect | POST | glz-eu-1.eu.a.pvp.net | `/session/v2/sessions/{id}/connect` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Pregame_GetPlayer | GET | glz-eu-1.eu.a.pvp.net | `/pregame/v1/players/{id}` |  | 10 | 200x9 404x1 | 16:48:24 | startup, before the tour |
| CoreGame_FetchPlayer | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/players/{id}` |  | 10 | 200x6 404x4 | 16:48:24 | startup, before the tour |
| Session_ReConnect | GET | glz-eu-1.eu.a.pvp.net | `/session/v1/sessions/{id}/reconnect` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| LoginQueue_FetchToken | POST | euc1.pp.sgp.pvp.net | `/login-queue/v2/login/products/valorant/regions/eu` |  | 1 | 200x1 | 16:48:24 | startup, before the tour |
| Premier_FetchPremierSeasons | GET | pd.eu.a.pvp.net | `/premier/v1/affinities/eu/premier-seasons` |  | 2 | 200x2 | 16:48:24 | startup, before the tour |
| Contracts_FetchContracts | GET | pd.eu.a.pvp.net | `/contracts/v1/contracts/{id}` |  | 7 | 200x7 | 16:48:25 | startup, before the tour |
| Store_GetGiftPurchaserEligibility | GET | pd.eu.a.pvp.net | `/store/v1/gifts/{id}/purchasereligibility` |  | 1 | 200x1 | 16:48:25 | startup, before the tour |
| Store_GetEntitlements | GET | pd.eu.a.pvp.net | `/store/v1/entitlements/{id}/{id}` |  | 8 | 200x8 | 16:48:25 | startup, before the tour |
| Contracts_FetchActiveRecruitmentEvent | GET | pd.eu.a.pvp.net | `/contracts/v1/agents/activeRecruitmentEvent` |  | 1 | 204x1 | 16:48:25 | startup, before the tour |
| Session_Heartbeat | POST | glz-eu-1.eu.a.pvp.net | `/session/v1/sessions/{id}/heartbeat` |  | 117 | 200x117 | 16:48:34 | startup, before the tour |
| PremierRosterService_GetPlayer_V3 | GET | pd.eu.a.pvp.net | `/premier/v3/players/{id}` |  | 1 | 200x1 | 16:48:36 | startup, before the tour |
| Party_FetchMUCToken | GET | glz-eu-1.eu.a.pvp.net | `/parties/v2/parties/{id}/muctoken` |  | 1 | 200x1 | 16:48:36 | startup, before the tour |
| Restrictions_FetchPlayerRestrictionsV3 | GET | pd.eu.a.pvp.net | `/restrictions/v3/penalties` |  | 1 | 200x1 | 16:48:36 | startup, before the tour |
| Restrictions_GetPlayerAvoidList | GET | pd.eu.a.pvp.net | `/restrictions/v1/avoidList` |  | 1 | 200x1 | 16:48:36 | startup, before the tour |
| Latency_Stats | POST | shared.eu.a.pvp.net | `/latency/v1/ingestMulti` |  | 15 | 204x15 | 16:48:36 | startup, before the tour |
| Party_FetchPlayer | GET | glz-eu-1.eu.a.pvp.net | `/parties/v1/players/{id}` | `preferredgamepods&<one key per server pod>` | 2 | 200x2 | 16:48:36 | startup, before the tour |
| Party_FetchParty | GET | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}` |  | 30 | 200x30 | 16:48:36 | startup, before the tour |
| Restrictions_GetPlayerInterventions | GET | pd.eu.a.pvp.net | `/restrictions/v1/activeFutureInterventions` |  | 1 | 200x1 | 16:48:37 | startup, before the tour |
| Store_GetStorefrontV3 | POST | pd.eu.a.pvp.net | `/store/v3/storefront/{id}` |  | 1 | 200x1 | 16:48:39 | startup, before the tour |
| Esports_GetLiveEventTimes | GET | pd.eu.a.pvp.net | `/esports-service/v2/liveEventTimes` | `locale&sport&leagueID&includePickemData` | 1 | 200x1 | 16:48:39 | startup, before the tour |
| MatchDetails_FetchMatchDetails | GET | pd.eu.a.pvp.net | `/match-details/v1/matches/{id}` |  | 27 | 200x27 | 16:51:07 | tour 1: Career, Match History |
| MatchHistory_FetchMatchHistory | GET | pd.eu.a.pvp.net | `/match-history/v1/history/{id}` | `startIndex&endIndex / startIndex&endIndex&queue` | 4 | 200x4 | 16:51:07 | tour 1: Career, Match History |
| SINGLE_SUMMARY | GET | d2oqczgbvomu1n.cloudfront.net | `/{id}/{id}.0.summary.json` | `Expires&Signature&Key-Pair-Id` | 19 |  | 16:51:08 | tour 1: Career, Match History |
| SINGLE_FILE_AVAILABILITY | HEAD | d3pl5tlo28h3c6.cloudfront.net | `/{id}/{id}.0.{id}.vrf` | `Expires&Signature&Key-Pair-Id` | 16 |  | 16:51:08 | tour 1: Career, Match History |
| MMR_FetchCompetitiveUpdates | GET | pd.eu.a.pvp.net | `/mmr/v1/players/{id}/competitiveupdates` | `startIndex&endIndex&queue` | 3 | 200x3 | 16:51:08 | tour 1: Career, Match History |
| BULK_SUMMARY | GET | euc1.pp.sgp.pvp.net | `/match-history-query/v3/products/valorant/players/{id}/infoTypes/SUMMARY` | `id (x7) / id (x9)` | 2 |  | 16:51:08 | tour 1: Career, Match History |
| BULK_FILE_AVAILABILITY | GET | euc1.pp.sgp.pvp.net | `/match-history-query/v3/products/valorant/players/{id}/infoTypes/REPLAY` | `id (x7) / id (x9)` | 2 |  | 16:51:08 | tour 1: Career, Match History |
| GoldStar_GetPlayer | GET | pd.eu.a.pvp.net | `/goldstars/v1/players/{id}` |  | 6 | 200x6 | 16:51:30 | tour 2: Career, competitive hub overview |
| RegionIndex_GetPlayer | GET | pd.eu.a.pvp.net | `/region-index/v1/players/{id}` | `seasonID` | 9 | 200x5 404x4 | 16:51:36 | tour 2: Career, competitive hub overview |
| Bloomline_FetchPlayerForBonsai | GET | pd.eu.a.pvp.net | `/bloomline/v0/players/{hex}/bonsai/{hex}` | `includeModules` | 1 | 200x1 | 16:52:40 | tour 3: Agents, mastery |
| DisplayNameService_FetchPlayers_BySubjects | PUT | pd.eu.a.pvp.net | `/name-service/v2/players` |  | 94 | 200x76 403x18 | 16:52:40 | tour 3: Agents, mastery |
| Esports_GetMatches | POST | pd.eu.a.pvp.net | `/esports-service/v2/matches` | `locale&sport` | 1 | 200x1 | 16:52:53 | Esports hub (opened instead of Premier, Leaderboards) |
| Esports_GetUpcomingMatches | GET | pd.eu.a.pvp.net | `/esports-service/v2/upcomingMatches` | `locale&sport&leagueID` | 1 | 200x1 | 16:52:53 | Esports hub (opened instead of Premier, Leaderboards) |
| Esports_GetOverview | GET | pd.eu.a.pvp.net | `/esports-service/v2/overview` | `locale&sport&leagueID` | 1 | 200x1 | 16:52:53 | Esports hub (opened instead of Premier, Leaderboards) |
| Party_RefreshPlayerIdentity | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/members/{id}/refreshPlayerIdentity` |  | 3 | 200x3 | 16:54:05 | after the tour: lobby, Career, Collections |
| playerLoadoutUpdate | PUT | pd.eu.a.pvp.net | `/personalization/v3/players/{id}/playerloadout` |  | 3 | 200x3 | 16:54:05 | after the tour: lobby, Career, Collections |
| Party_ChangeQueue | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/queue` |  | 2 | 200x2 | 16:57:32 | match 1 (Swift): queue, agent select and the match |
| Party_EnterMatchmakingQueue | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/matchmaking/join` |  | 3 | 200x3 | 16:57:42 | match 1 (Swift): queue, agent select and the match |
| Pregame_GetMatchLoadouts | GET | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}/loadouts` |  | 3 | 200x3 | 16:57:54 | match 1 (Swift): queue, agent select and the match |
| Pregame_FetchChatToken | GET | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}/chattoken` |  | 6 | 200x6 | 16:57:54 | match 1 (Swift): queue, agent select and the match |
| Pregame_GetMatch | GET | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}` |  | 76 | 200x70 404x6 | 16:57:54 | match 1 (Swift): queue, agent select and the match |
| Pregame_FetchVoiceToken | GET | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}/voicetoken` |  | 3 | 200x3 | 16:57:59 | match 1 (Swift): queue, agent select and the match |
| Pregame_LockCharacter | POST | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}/lock/{id}` |  | 3 | 200x3 | 16:58:09 | match 1 (Swift): queue, agent select and the match |
| Pregame_SelectCharacter | POST | glz-eu-1.eu.a.pvp.net | `/pregame/v1/matches/{id}/select/{id}` |  | 3 | 200x3 | 16:58:09 | match 1 (Swift): queue, agent select and the match |
| CoreGame_FetchAllChatMUCToken | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/matches/{id}/allchatmuctoken` |  | 3 | 200x3 | 16:58:33 | match 1 (Swift): queue, agent select and the match |
| CoreGame_FetchMatchLoadouts | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/matches/{id}/loadouts` |  | 3 | 200x3 | 16:58:33 | match 1 (Swift): queue, agent select and the match |
| CoreGame_FetchMatch | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/matches/{id}` |  | 13 | 200x13 | 16:58:33 | match 1 (Swift): queue, agent select and the match |
| CoreGame_FetchTeamChatMUCToken | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/matches/{id}/teamchatmuctoken` |  | 3 | 200x3 | 16:58:33 | match 1 (Swift): queue, agent select and the match |
| Latency_Stat | POST | shared.eu.a.pvp.net | `/latency/v1/ingest` |  | 23 | 204x23 | 17:01:04 | match 1 (Swift): queue, agent select and the match |
| Party_SetMemberReady | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/members/{id}/setReady` |  | 3 | 200x3 | 17:12:56 | match 1 (Swift): after the match, end screens, menus |
| Contracts_FetchProcessedMatch | GET | pd.eu.a.pvp.net | `/contracts/v1/players/{id}/matches/{id}` |  | 3 | 200x3 | 17:12:59 | match 1 (Swift): after the match, end screens, menus |
| Bloomline_FetchPlayerForBonsai | GET | pd.eu.a.pvp.net | `/bloomline/v0/players/{id}/bonsai/{hex}` |  | 3 | 200x3 | 17:12:59 | match 1 (Swift): after the match, end screens, menus |
| DailyRewards_GET | GET | pd.eu.a.pvp.net | `/daily-ticket/v1/{id}` |  | 3 | 200x3 | 17:12:59 | match 1 (Swift): after the match, end screens, menus |
| Bloomline_FetchProcessedMatch | GET | pd.eu.a.pvp.net | `/bloomline/v0/players/{id}/matches/{hex}` |  | 3 | 200x3 | 17:12:59 | match 1 (Swift): after the match, end screens, menus |
| MatchDetails_FetchMatchDetails | GET | pd.eu.a.pvp.net | `/match-details/v1/matches/{hex}` |  | 6 | 200x6 | 17:13:01 | match 1 (Swift): after the match, end screens, menus |
| Party_RefreshCompetitiveTier | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/members/{id}/refreshCompetitiveTier` |  | 3 | 200x3 | 17:13:01 | match 1 (Swift): after the match, end screens, menus |
| CoreGame_DisassociatePlayer | POST | glz-eu-1.eu.a.pvp.net | `/core-game/v1/players/{id}/disassociate/{id}` |  | 3 | 204x3 | 17:13:01 | match 1 (Swift): after the match, end screens, menus |

## Seen only in today's earlier log (first session, 10:43â€“15:04 UTC)

These come from `ShooterGame-backup-2026.10.27-15.04.45.log`, the morning's session with several matches. They aren't in this session's log yet.

| Query name | Method | Host | Path | Query keys | Count | Codes | First seen |
|---|---|---|---|---|---|---|---|
| Party_StartSoloExperience | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/players/{id}/startsoloexperience` |  | 2 | 200x2 | 10:50:28 |
| CoreGame_FetchVoiceToken | GET | glz-eu-1.eu.a.pvp.net | `/core-game/v1/matches/{id}/teamvoicetoken` |  | 3 | 200x3 | 10:50:28 |
| Premier_GetPlayerRoster_PublicInfo | GET | pd.eu.a.pvp.net | `/premier/v3/players/{id}/playerRosterPublicInfo` |  | 8 | 200x8 | 11:20:00 |
| Bloomline_FetchPlayer | GET | pd.eu.a.pvp.net | `/bloomline/v0/players/{hex}` |  | 5 | 200x5 | 11:20:00 |
| Party_InviteToParty | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/parties/{id}/invites` |  | 1 | 200x1 | 11:27:36 |
| Party_FetchVoiceToken | GET | glz-eu-1.eu.a.pvp.net | `/parties/v2/parties/{id}/voicetoken` |  | 2 | 200x2 | 11:27:39 |
| Party_PlayerJoin | POST | glz-eu-1.eu.a.pvp.net | `/parties/v1/players/{id}/joinparty/{id}` |  | 1 | 200x1 | 11:43:56 |
| Restrictions_GetInterventionFeedback | GET | pd.eu.a.pvp.net | `/restrictions/v1/interventionFeedback/{id}` |  | 1 | 200x1 | 11:48:11 |
| Premier_GetEligibility | GET | pd.eu.a.pvp.net | `/premier/v1/player/eligibility` |  | 2 | 200x2 | 12:07:24 |
| Premier_GetMatchLimitInfo | GET | pd.eu.a.pvp.net | `/premier/v1/matchlimitinfo` |  | 1 | 200x1 | 12:07:24 |

## Messaging service pushes

| Service path | Earlier log count | This session count | First seen this session |
|---|---|---|---|
| `activitygateway/activities/v1/{code}/member-state` | 3 | 1 | 16:48:38 |
| `activitygateway/activities/v1/joined` | 2 | 1 | 16:48:38 |
| `ares-accolades/accolades/v1/players/{id}` | 4 | 0 |  |
| `ares-account-xp/account-xp/v1/players/{id}` | 9 | 3 | 17:12:59 |
| `ares-contracts/contracts/v1/players/{id}/matches` | 9 | 3 | 17:12:59 |
| `ares-core-game/core-game/v1/matches/{id}` | 128 | 36 | 17:12:59 |
| `ares-core-game/core-game/v1/players/{id}` | 22 | 6 | 16:58:33 |
| `ares-daily-ticket/daily-ticket/v1/{id}` | 9 | 3 | 17:12:59 |
| `ares-mastery/bloomline/v0/players/{id}/matches` | 5 | 3 | 17:12:59 |
| `ares-match-details/match-details/v1/matches` | 9 | 3 | 17:12:59 |
| `ares-mmr/mmr/v1/players/{id}` | 9 | 3 | 17:12:59 |
| `ares-parties/parties/v1/parties/{id}` | 112 | 29 | 16:54:05 |
| `ares-parties/parties/v1/players/{id}` | 5 | 1 | 16:48:36 |
| `ares-pregame/pregame/v1/matches/{id}` | 146 | 66 | 16:58:06 |
| `ares-pregame/pregame/v1/players/{id}` | 15 | 9 | 16:57:54 |
| `ares-region-index/region-index/v1/players/{id}` | 4 | 0 |  |
| `ares-restrictions/restrictions/v1/interventionFeedback` | 1 | 0 |  |
| `ares-session/v1/sessions/{id}` | 28 | 10 | 16:48:24 |
| `cap/progression/v1/notifications/cache/invalidate` | 42 | 18 | 17:12:59 |
| `cap/stats/v1/DeltaEvent/{id}` | 9 | 3 | 17:12:59 |
| `cap/v1/wallets` | 9 | 3 | 17:12:59 |
| `gdu/v1/product/VALORANT/match/infoType/REPLAY` | 4 | 3 | 17:13:00 |
| `gdu/v1/product/VALORANT/match/infoType/SUMMARY` | 4 | 3 | 17:13:00 |
