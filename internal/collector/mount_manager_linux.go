//go:build linux
// +build linux

package collector

import (
	"bufio"
	"io"
	"os"
	"strings"
)

func parseMounts() ([]MountInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseMountsFrom(f)
}

func parseMountsFrom(r io.Reader) ([]MountInfo, error) {
	var mounts []MountInfo
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		m := MountInfo{
			Device:     fields[0],
			Mountpoint: decodeMountPath(fields[1]),
			FSType:     fields[2],
		}

		if shouldIgnore(m) {
			continue
		}

		mounts = append(mounts, m)
	}

	return mounts, scanner.Err()
}

// decodeMountPath replaces common octal escapes in /proc/mounts.
func decodeMountPath(s string) string {
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
}
