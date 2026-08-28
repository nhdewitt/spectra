//go:build windows

package memory

// SwapPaging is not implemented on Windows and reports nothing.
func SwapPaging() (*float64, *float64, error) {
	return nil, nil, nil
}
