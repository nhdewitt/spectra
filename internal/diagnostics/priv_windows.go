//go:build windows

package diagnostics

import "golang.org/x/sys/windows"

func isPrivileged() bool {
	ok, err := isWindowsAdmin()
	return err == nil && ok
}

func isWindowsAdmin() (bool, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()

	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, err
	}

	isMember, err := token.IsMember(adminSID)
	if err != nil {
		return false, err
	}

	return isMember, nil
}
