package app

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/glitchfl/lowkey/internal/remote"
	"github.com/glitchfl/lowkey/internal/static"
)

// staticRefresh remembers which unknown IDs a finished refresh already looked for - once per run each
// whether it worked or not - a refresh a dodge or the cutoff cancelled never finished so it can go again
type staticRefresh struct {
	asked      map[string]bool
	agentsBusy bool
	mapsBusy   bool
	skinsBusy  map[string]bool
	gen        int
}

// refreshStatic asks valorant-api.com about agents or maps or skins the snapshot doesn't know - only
// inside a window
func (lv *live) refreshStatic() {
	if lv.lobby == nil || lv.guard == nil || lv.eng.sched == nil {
		return
	}
	if w := lv.guard.Window(); w != remote.AgentSelect && w != remote.MatchLoad {
		return
	}
	// a cancelled refresh never replies - its busy flags belong to the generation it died with
	if lv.static.gen != lv.eng.gen {
		lv.static.gen, lv.static.agentsBusy, lv.static.mapsBusy = lv.eng.gen, false, false
		lv.static.skinsBusy = map[string]bool{}
	}
	var agents []string
	unknown := func(id string) {
		if _, ok := lv.data.AgentName(id); id != "" && !ok && !lv.static.asked[id] {
			agents = append(agents, id)
		}
	}
	for _, m := range lv.lobby.members {
		unknown(m.agent)
		if m.p != nil {
			for _, match := range m.p.loaded() {
				if p := match.Players[m.puuid]; p != nil {
					unknown(p.Agent)
				}
			}
		}
	}
	if len(agents) > 0 && !lv.static.agentsBusy {
		lv.static.agentsBusy = true
		q := url.Values{"isPlayableCharacter": {"true"}}
		lv.eng.submit(&job{prio: prioRoster, svc: remote.Static, path: "/v1/agents", query: q}, func(body []byte, err error) {
			lv.static.agentsBusy = false
			for _, id := range agents {
				lv.static.asked[id] = true
			}
			if err == nil {
				lv.data.AddAgents(decodeStaticAgents(body))
			}
		})
	}
	mapURL := lv.lobby.mapURL
	if _, ok := lv.data.MapName(mapURL); mapURL != "" && !ok && !lv.static.asked[mapURL] && !lv.static.mapsBusy {
		lv.static.mapsBusy = true
		lv.eng.submit(&job{prio: prioRoster, svc: remote.Static, path: "/v1/maps"}, func(body []byte, err error) {
			lv.static.mapsBusy = false
			lv.static.asked[mapURL] = true
			if err == nil {
				lv.data.AddMaps(decodeStaticMaps(body))
			}
		})
	}
	// skins are thousands so each new one is asked for by itself - only the guns on screen and only
	// for players whose skins can show
	for _, gun := range lv.cfg.Settings().Weapons {
		weapon, weaponName, ok := lv.data.Weapon(gun)
		if !ok {
			continue
		}
		for _, m := range lv.lobby.members {
			id := lv.lobby.skins[m.puuid][weapon]
			if id == "" || m.p == nil || lv.static.asked[id] || lv.static.skinsBusy[id] {
				continue
			}
			if _, known := lv.data.Skin(id); known {
				continue
			}
			lv.static.skinsBusy[id] = true
			lv.eng.submit(&job{prio: prioRoster, svc: remote.Static, path: "/v1/weapons/skins/" + id}, func(body []byte, err error) {
				delete(lv.static.skinsBusy, id)
				lv.static.asked[id] = true
				if s, ok := decodeStaticSkin(body, weaponName); err == nil && ok {
					lv.data.AddSkins(map[string]static.Skin{id: s})
				}
			})
		}
	}
}

func decodeStaticAgents(body []byte) map[string]string {
	var v struct {
		Data []struct {
			UUID        string `json:"uuid"`
			DisplayName string `json:"displayName"`
		} `json:"data"`
	}
	out := map[string]string{}
	if json.Unmarshal(body, &v) == nil {
		for _, a := range v.Data {
			if a.UUID != "" && a.DisplayName != "" {
				out[strings.ToLower(a.UUID)] = a.DisplayName
			}
		}
	}
	return out
}

// decodeStaticSkin names a skin the way the snapshot does - Reaver for Reaver Vandal
func decodeStaticSkin(body []byte, weapon string) (static.Skin, bool) {
	var v struct {
		Data struct {
			DisplayName     string `json:"displayName"`
			ContentTierUUID string `json:"contentTierUuid"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &v) != nil || strings.TrimSpace(v.Data.DisplayName) == "" {
		return static.Skin{}, false
	}
	name := strings.TrimSuffix(strings.TrimSpace(v.Data.DisplayName), " "+weapon)
	return static.Skin{Name: name, Edition: strings.ToLower(v.Data.ContentTierUUID)}, true
}

func decodeStaticMaps(body []byte) map[string]string {
	var v struct {
		Data []struct {
			DisplayName string `json:"displayName"`
			MapURL      string `json:"mapUrl"`
		} `json:"data"`
	}
	out := map[string]string{}
	if json.Unmarshal(body, &v) == nil {
		for _, m := range v.Data {
			if m.MapURL != "" && m.DisplayName != "" {
				out[m.MapURL] = m.DisplayName
			}
		}
	}
	return out
}
