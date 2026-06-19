package server

import (
	"net/http"
	"strings"
)

// platformFromOS maps an agent's OS/arch to the matching platformInfo.
func platformFromAgent(os, arch string) *platformInfo {
	os = strings.ToLower(os)
	arch = strings.ToLower(arch)

	for i, p := range knownPlatforms {
		if p.OS == os && p.Arch == arch {
			return &knownPlatforms[i]
		}
		if p.OS == os && p.Arch == "arm" && (arch == "armv61" || arch == "armv71") {
			variant := strings.TrimSuffix(arch, "1")
			if p.Variant == variant {
				return &knownPlatforms[i]
			}
		}
	}

	return nil
}

func (s *Server) handleUpgradeInstructions(w http.ResponseWriter, r *http.Request) {
	agentID, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	agent, err := s.DB.GetAgent(r.Context(), mustUUID(agentID))
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	if agent.Os.String == "" || agent.Arch.String == "" {
		http.Error(w, "no platform match for empty OS/arch", http.StatusBadRequest)
		return
	}

	p := platformFromAgent(agent.Os.String, agent.Arch.String)
	if p == nil {
		respondJSON(w, http.StatusOK, upgradeInstructions{})
		return
	}

	instructions := generateUpgradeInstructions(p)
	respondJSON(w, http.StatusOK, instructions)
}

func (s *Server) handleUninstallInstructions(w http.ResponseWriter, r *http.Request) {
	agentID, err := parsePathID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	agent, err := s.DB.GetAgent(r.Context(), mustUUID(agentID))
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	if agent.Os.String == "" || agent.Arch.String == "" {
		http.Error(w, "no platform match for empty OS/arch", http.StatusBadRequest)
		return
	}

	p := platformFromAgent(agent.Os.String, agent.Arch.String)
	if p == nil {
		respondJSON(w, http.StatusOK, uninstallInstructions{})
		return
	}

	instructions := generateUninstallInstructions(p)
	respondJSON(w, http.StatusOK, instructions)
}
