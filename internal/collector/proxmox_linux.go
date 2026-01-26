//go:build !windows

package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nhdewitt/spectra/internal/protocol"
)

const (
	proxmoxSource = "proxmox"
	kindLXC       = "lxc"
	kindVM        = "vm"

	proxmoxConcurrency = 32
)

var (
	cachedNode     string
	cachedNodeOnce sync.Once
	cachedNodeErr  error
)

type pveNode struct {
	Node string `json:"node"`
}

type proxmoxListRow struct {
	VMID   int    `json:"vmid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type proxmoxResource struct {
	Type   string  `json:"type"` // "lxc", "qemu", "node", "storage", "sdn"
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"` // "running", "stopped", ...
	CPU    float64 `json:"cpu"`    // fraction
	CPUs   int     `json:"cpus"`   // assigned cores
	Mem    uint64  `json:"mem"`    // bytes
	MaxMem uint64  `json:"maxmem"` // bytes
	NetIn  uint64  `json:"netin"`  // bytes
	NetOut uint64  `json:"netout"` // bytes
}

func localProxmoxNode(ctx context.Context) (string, error) {
	cachedNodeOnce.Do(func() {
		cachedNode, cachedNodeErr = resolveProxmoxNode(ctx)
	})
	return cachedNode, cachedNodeErr
}

func resolveProxmoxNode(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pvesh", "get", "/nodes", "--output-format", "json")

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var nodes []pveNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		return "", err
	}

	hn, _ := os.Hostname()
	hn = strings.TrimSpace(hn)

	for _, n := range nodes {
		if strings.EqualFold(n.Node, hn) {
			return n.Node, nil
		}
	}

	if len(nodes) == 1 {
		return nodes[0].Node, nil
	}

	return "", fmt.Errorf("unable to resolve proxmox node name")
}

func collectProxmoxGuests(ctx context.Context) ([]protocol.ContainerMetric, error) {
	if !hasCommand("pvesh") {
		return nil, nil
	}

	node, err := localProxmoxNode(ctx)
	if err != nil {
		return nil, err
	}

	lxcs, lxcErr := proxmoxList(ctx, node, kindLXC)
	vms, vmErr := proxmoxList(ctx, node, kindVM)
	if lxcErr != nil && vmErr != nil {
		return nil, fmt.Errorf("proxmox list failed (lxc: %v, qemu: %v)", lxcErr, vmErr)
	}

	var out []protocol.ContainerMetric

	out = append(out, collectProxmoxKind(ctx, node, kindLXC, lxcs)...)
	out = append(out, collectProxmoxKind(ctx, node, kindVM, vms)...)

	return out, nil
}

func collectProxmoxKind(ctx context.Context, node, kind string, rows []proxmoxListRow) []protocol.ContainerMetric {
	type result struct {
		m  protocol.ContainerMetric
		ok bool
	}

	results := make(chan result, len(rows))
	sem := make(chan struct{}, proxmoxConcurrency)

	for _, row := range rows {
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			vmid := strconv.Itoa(row.VMID)
			if r, ok := proxmoxStatus(ctx, node, kind, vmid); ok {
				results <- result{
					m:  mapProxmoxStatus(r, kind),
					ok: true,
				}
				return
			}
			results <- result{ok: false}
		}()
	}

	var out []protocol.ContainerMetric
	for range rows {
		r := <-results
		if r.ok {
			out = append(out, r.m)
		}
	}

	return out
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func proxmoxList(ctx context.Context, node, kind string) ([]proxmoxListRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var path string
	switch kind {
	case kindLXC:
		path = "/nodes/" + node + "/lxc"
	case kindVM:
		path = "/nodes/" + node + "/qemu"
	default:
		return nil, fmt.Errorf("unknown kind %q", kind)
	}

	cmd := exec.CommandContext(ctx, "pvesh", "get", path, "--output-format", "json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ctx.Err()
		}
		if stderr.Len() > 0 {
			return nil, errors.New(stderr.String())
		}
		return nil, err
	}

	var rows []proxmoxListRow
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func proxmoxStatus(ctx context.Context, node, kind, vmid string) (proxmoxResource, bool) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var path string
	switch kind {
	case kindLXC:
		path = "/nodes/" + node + "/lxc/" + vmid + "/status/current"
	case kindVM:
		path = "/nodes/" + node + "/qemu/" + vmid + "/status/current"
	default:
		return proxmoxResource{}, false
	}

	cmd := exec.CommandContext(ctx, "pvesh", "get", path, "--output-format", "json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return proxmoxResource{}, false
		}
		if stderr.Len() > 0 {
			return proxmoxResource{}, false
		}
		return proxmoxResource{}, false
	}

	var out proxmoxResource
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return proxmoxResource{}, false
	}
	return out, true
}

func mapProxmoxStatus(r proxmoxResource, kind string) protocol.ContainerMetric {
	cores := uint32(0)
	if r.CPUs > 0 {
		cores = uint32(r.CPUs)
	}

	cpuPct := 0.0
	if r.CPU > 0 && r.CPUs > 0 {
		cpuPct = r.CPU * float64(r.CPUs) * 100.0
	}

	return protocol.ContainerMetric{
		ID:            strconv.Itoa(r.VMID),
		Name:          r.Name,
		State:         r.Status,
		Source:        proxmoxSource,
		Kind:          kind,
		CPUPercent:    cpuPct,
		CPULimitCores: cores,
		MemoryBytes:   r.Mem,
		MemoryLimit:   r.MaxMem,
		NetRxBytes:    r.NetIn,
		NetTxBytes:    r.NetOut,
	}
}
