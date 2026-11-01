# 03: What the client shows about a player's performance

**Why:** Competitive match details carry Riot's own performance grade under made-up names: a 0–500 score, a `pass` / `merit` / `distinction` grade, and up/down ratings for damage, kill impact, death impact, trades, assists, utility, plants and defuses (`.scratch/data-discovery/findings.md`). The decoding is statistical. Seeing what the client shows for the same match confirms it, and tells whether players already see this about themselves. The same goes for the agent mastery level that pregame and core-game carry.

**Needs:** a live match the user queues, Competitive if they're willing, with lowkey v0.1.2 recording

**Status:** open

## Steps

1. During agent select, ask nothing. Afterwards, ask whether the client showed a mastery level for anyone's agent, theirs included, and what it was.
2. When the match ends, ask the user to look through the end-of-match screens before leaving them, then answer in one message:
   - any grade or rating word shown for them, such as Pass, Merit, Distinction or anything like it, and on which screen
   - any up or down arrows, or ratings, for damage, kill impact, death impact, trades, assists, utility, plants or defuses: which ones and which way
   - the RR change, and whether the client split it into parts, such as a performance bonus
   - the mastery level or progress shown for the agent they played
   - anything about a map rating or a per-map score
   - whether the client showed grades for other players too
3. Note the match's end time and the agent they played, so the Mac can find the match in lowkey's recording. lowkey's session refresh downloads that match's details within seconds of the match ending.
4. If they played another mode, note it anyway. Only competitive matches had these fields so far.

## Send back

In the report, per match: the user's answers, word for word where they quote the client, plus the match's end time and agent.

## Results
