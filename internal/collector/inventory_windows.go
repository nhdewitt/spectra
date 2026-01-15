//go:build windows

package collector

import (
	"context"

	"github.com/nhdewitt/spectra/internal/protocol"
	"golang.org/x/sys/windows/registry"
)

func GetInstalledApps(ctx context.Context) ([]protocol.Application, error) {
	apps := make([]protocol.Application, 0, 100)

	// Need to check two registry keys (32-bit + 64-bit)
	keys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}

	for _, path := range keys {
		// Use a closure to ensure the Close() runs immediately after
		func() {
			k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.ENUMERATE_SUB_KEYS|registry.READ)
			if err != nil {
				return
			}
			defer k.Close()

			subkeys, err := k.ReadSubKeyNames(-1)
			if err != nil {
				return
			}

			for _, subkeyName := range subkeys {
				sk, err := registry.OpenKey(registry.LOCAL_MACHINE, path+"\\"+subkeyName, registry.READ)
				if err != nil {
					continue
				}

				name, _, err := sk.GetStringValue("DisplayName")
				if err == nil && name != "" {
					version, _, _ := sk.GetStringValue("DisplayVersion")
					publisher, _, _ := sk.GetStringValue("Publisher")

					apps = append(apps, protocol.Application{
						Name:    name,
						Version: version,
						Vendor:  publisher,
					})
				}
				sk.Close()
			}
		}()
	}

	return apps, nil
}
