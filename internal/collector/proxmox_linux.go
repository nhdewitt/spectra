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
	typeLXC       = "lxc"
	typeQEMU      = "qemu"
)

var (
	cachedNode     string
	cachedNodeOnce sync.Once
	cachedNodeErr  error
)

type pveNode struct {
	Node string `json:"node"`
}

type proxmoxClusterRow struct {
	ID     string  `json:"id"`   // "lxc/<vmid>" or "qemu/<vmid>"
	Type   string  `json:"type"` // "lxc" or "qemu"
	Node   string  `json:"node"`
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MaxCPU int     `json:"maxcpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	NetIn  uint64  `json:"netin"`
	NetOut uint64  `json:"netout"`
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

	rows, err := proxmoxClusterResources(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]protocol.ContainerMetric, 0, len(rows))
	for _, r := range rows {
		if r.Node != node {
			continue
		}
		if r.Type != typeLXC && r.Type != typeQEMU {
			continue
		}

		var kind string
		switch r.Type {
		case typeLXC:
			kind = kindLXC
		case typeQEMU:
			kind = kindVM
		}

		cores := uint32(0)
		if r.MaxCPU > 0 {
			cores = uint32(r.MaxCPU)
		}

		cpuPct := 0.0
		if r.CPU > 0 && r.MaxCPU > 0 {
			cpuPct = r.CPU * float64(r.MaxCPU) * 100.0
		}

		out = append(out, protocol.ContainerMetric{
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
		})
	}

	return out, nil
}

func proxmoxClusterResources(ctx context.Context) ([]proxmoxClusterRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pvesh", "get", "/cluster/resources", "--type", "vm", "--output-format", "json")

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

	var rows []proxmoxClusterRow
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		return nil, err
	}

	return rows, nil
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
