package pprofd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// maxUploadBytes caps a single POST.
const maxUploadBytes = 32 << 20

type Server struct {
	store  *Store
	log    *slog.Logger
	Router *http.ServeMux
}

func NewServer(store *Store, log *slog.Logger) *Server {
	s := &Server{store: store, log: log, Router: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.Router.HandleFunc("POST /v1/profiles", s.handleUpload)
	s.Router.HandleFunc("GET /v1/agents", s.handleAgents)
	s.Router.HandleFunc("GET /v1/summary", s.handleSummary)
	s.Router.HandleFunc("GET /v1/agents/{agent}/profiles", s.handleList)
	s.Router.HandleFunc("GET /v1/agents/{agent}/profiles/{id}", s.handleProfile)
	s.Router.HandleFunc("GET /healthz", s.handleHealth)
	s.Router.HandleFunc("GET /", s.handleIndex)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	var up Upload
	if err := json.NewDecoder(r.Body).Decode(&up); err != nil {
		s.log.Warn("rejected upload", "error", err, "remote", r.RemoteAddr)
		respondError(w, http.StatusBadRequest, "malformed upload")
		return
	}

	id, err := s.store.Put(up)
	if err != nil {
		s.log.Warn("could not store upload", "error", err, "agent", up.AgentID)
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.log.Info("stored profile",
		"agent", up.AgentID,
		"host", up.Hostname,
		"kind", up.Kind,
		"arch", up.Arch,
		"bytes", len(up.Profile),
	)

	respondJSON(w, http.StatusAccepted, map[string]string{"id": id})
}

func (s *Server) handleAgents(w http.ResponseWriter, _ *http.Request) {
	agents, err := s.store.Agents()
	if err != nil {
		s.log.Error("could not list agents", "error", err)
		respondError(w, http.StatusInternalServerError, "could not list agents")
		return
	}
	if agents == nil {
		agents = []string{}
	}
	respondJSON(w, http.StatusOK, agents)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	recs, err := s.store.List(r.PathValue("agent"))
	if errors.Is(err, ErrNotFound) {
		respondError(w, http.StatusNotFound, "no such agent")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if recs == nil {
		recs = []Record{}
	}
	respondJSON(w, http.StatusOK, recs)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	agent := r.PathValue("agent")
	id := r.PathValue("id")
	id = strings.TrimSuffix(id, ".pprof")

	data, err := s.store.Profile(agent, id)
	if errors.Is(err, ErrNotFound) {
		respondError(w, http.StatusNotFound, "no such profile")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", id+".pprof"))
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.Write(data)
}

func (s *Server) handleSummary(w http.ResponseWriter, _ *http.Request) {
	sums, err := s.store.Summaries()
	if err != nil {
		s.log.Error("could not summarize", "error", err)
		respondError(w, http.StatusInternalServerError, "could not summarize")
		return
	}
	if sums == nil {
		sums = []Summary{}
	}
	respondJSON(w, http.StatusOK, sums)
}

func writeComparison(b *strings.Builder, sums []Summary) {
	if len(sums) == 0 {
		return
	}

	b.WriteString("<h2>Across agents</h2>")
	b.WriteString("<table><tr><th>Host</th><th>CPU</th><th>Arch</th><th>Version</th>")
	b.WriteString("<th>RAM</th><th>Limit</th><th>Heap live</th><th>of limit</th>")
	b.WriteString("<th>GC CPU/s</th><th>GC/s</th><th>Goroutines</th><th>Last seen</th></tr>")

	for _, sum := range sums {
		fmt.Fprintf(b,
			"<tr><td><a href=\"#%s\">%s</a></td><td>%s &times;%d</td><td>%s/%s%s</td><td>%s</td>"+
				"<td>%s</td><td>%s%s</td><td>%s</td><td>%.1f%%</td>"+
				"<td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>",
			htmlEscape(sum.AgentID), htmlEscape(sum.Hostname),
			htmlEscape(sum.CPUModel), sum.CPUCores,
			htmlEscape(sum.OS), htmlEscape(sum.Arch), cgoNote(sum.CGO),
			htmlEscape(sum.AgentVersion),
			humanBytes(sum.RAMTotalBytes),
			humanBytes(uint64(max(sum.GOMEMLIMIT, 0))), clampNote(sum),
			humanBytes(sum.HeapLiveBytes),
			sum.HeapHeadroom*100,
			rateCell(sum.GCCPUFraction, "%.3f"),
			rateCell(sum.GCRate, "%.2f"),
			sum.Goroutines,
			sum.LastSeen.Format(time.RFC3339),
		)
	}
	b.WriteString("</table>")
}

func clampNote(sum Summary) string {
	if sum.LimitClamped {
		return " <em>(clamped)</em>"
	}
	if sum.LimitPctOfRAM > 0 {
		return fmt.Sprintf(" (%.0f%% RAM)", sum.LimitPctOfRAM*100)
	}
	return ""
}

func cgoNote(cgo bool) string {
	if cgo {
		return " +cgo"
	}
	return ""
}

func rateCell(v *float64, format string) string {
	if v == nil {
		return "&mdash;"
	}
	return fmt.Sprintf(format, *v)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		respondError(w, http.StatusNotFound, "not found")
		return
	}

	agents, err := s.store.Agents()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not list agents")
		return
	}

	var b strings.Builder
	b.WriteString("<!doctype html><meta charset=utf-8><title>Spectra profiles</title>")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem;max-width:70rem}")
	b.WriteString("table{border-collapse:collapse;width:100%;margin-bottom:2rem}")
	b.WriteString("th,td{text-align:left;padding:.3rem .6rem;border-bottom:1px solid #ddd;font-size:.9rem}")
	b.WriteString("code{font-size:.85rem}")
	b.WriteString("button.copy{font:inherit;font-size:.8rem;padding:.1rem .4rem;cursor:pointer;")
	b.WriteString("border:1px solid #bbb;border-radius:3px;background:#f6f6f6}")
	b.WriteString("button.copy.ok{background:#d8f0d8;border-color:#8c8}</style>")
	b.WriteString("<h1>Spectra profiles</h1>")
	b.WriteString("<p><label><input type=checkbox id=httpmode> ")
	b.WriteString("open in browser (adds <code>-http=:</code>)</label></p>")

	if len(agents) == 0 {
		b.WriteString("<p>No profiles collected yet.</p>")
	}

	if sums, err := s.store.Summaries(); err == nil {
		writeComparison(&b, sums)
	}

	for _, agent := range agents {
		recs, err := s.store.List(agent)
		if err != nil || len(recs) == 0 {
			continue
		}

		head := recs[0]
		fmt.Fprintf(&b, "<h2 id=%q>%s</h2>", htmlEscape(agent), htmlEscape(head.Hostname))
		fmt.Fprintf(&b,
			"<p><code>%s</code> &middot; %s/%s &middot; %s &times;%d &middot; %s (%s) &middot; %s &middot; cgo=%t</p>",
			htmlEscape(agent),
			htmlEscape(head.OS), htmlEscape(head.Arch),
			htmlEscape(head.CPUModel), head.CPUCores,
			htmlEscape(head.AgentVersion), htmlEscape(head.AgentCommit),
			htmlEscape(head.GoVersion), head.CGO,
		)

		b.WriteString("<table><tr><th>Captured</th><th>Kind</th><th>Heap live</th>")
		b.WriteString("<th>GOMEMLIMIT</th><th>GC cycles</th><th>GC CPU</th><th>Goroutines</th><th></th></tr>")

		for _, rec := range recs {
			fmt.Fprintf(&b,
				"<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%.2fs</td><td>%d</td>"+
					"<td><a href=\"/v1/agents/%s/profiles/%s\">download</a>%s</td></tr>",
				rec.CapturedAt.Format(time.RFC3339),
				htmlEscape(rec.Kind),
				humanBytes(rec.Runtime.HeapLiveBytes),
				humanBytes(uint64(max(rec.Runtime.GOMEMLIMIT, 0))),
				rec.Runtime.GCCycles,
				rec.Runtime.GCCPUSecs,
				rec.Runtime.Goroutines,
				htmlEscape(agent), htmlEscape(rec.ID),
				copyButtons(rec.Kind),
			)
		}
		b.WriteString("</table>")
	}

	b.WriteString(copyScript)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(b.String()))
}

func copyButtons(kind string) string {
	views := []struct{ label, flags string }{{"copy", ""}}

	if kind == "heap" {
		views = []struct{ label, flags string }{
			{"inuse", "-sample_index=inuse_space"},
			{"inuse obj", "-sample_index=inuse_objects"},
			{"alloc", "-sample_index=alloc_space"},
			{"alloc obj", "-sample_index=alloc_objects"},
		}
	}

	var b strings.Builder
	for _, v := range views {
		fmt.Fprintf(&b, " <button class=copy type=button data-flags=%q>%s</button>",
			v.flags, htmlEscape(v.label))
	}
	return b.String()
}

const copyScript = `<script>
document.addEventListener("click", function (ev) {
  var btn = ev.target.closest("button.copy");
  if (!btn) return;
 
  var link = btn.parentNode.querySelector("a");
  if (!link) return;
 
  var parts = ["go tool pprof"];
 
  var httpMode = document.getElementById("httpmode");
  if (httpMode && httpMode.checked) parts.push("-http=:");
 
  var flags = btn.getAttribute("data-flags");
  if (flags) parts.push(flags);
 
  parts.push(location.origin + link.getAttribute("href"));
  var cmd = parts.join(" ");
 
  function done() {
    var was = btn.textContent;
    btn.textContent = "copied";
    btn.classList.add("ok");
    setTimeout(function () {
      btn.textContent = was;
      btn.classList.remove("ok");
    }, 1200);
  }
 
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(cmd).then(done, function () { fallback(cmd, done); });
    return;
  }
  fallback(cmd, done);
});
 
function fallback(text, done) {
  var ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand("copy");
    done();
  } catch (e) {
    window.prompt("Copy:", text);
  }
  document.body.removeChild(ta);
}
</script>`

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
