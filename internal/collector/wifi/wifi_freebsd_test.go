//go:build freebsd

package wifi

import (
	"bytes"
	"fmt"
	"testing"
)

const ifconfigAssociated = `wlan0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> metric 0 mtu 1500
	ether f4:96:34:3f:38:60
	inet 192.168.1.42 netmask 0xffffff00 broadcast 192.168.1.255
	groups: wlan
	ssid "MyNetwork" channel 149 (5745 MHz 11a)
	regdomain FCC country US authmode WPA2/802.11i privacy ON
	txpower 30 bmiss 10 scanvalid 60 wme bintval 0
	parent interface: iwm0
	media: IEEE 802.11 Wireless Ethernet MCS mode 11na
	status: associated
	nd6 options=29<PERFORMNUD,IFDISABLED,AUTO_LINKLOCAL>
`

const ifconfigNotAssociated = `wlan0: flags=8802<BROADCAST,SIMPLEX,MULTICAST> metric 0 mtu 1500
	options=0
	ether f4:96:34:3f:38:60
	groups: wlan
	ssid "" channel 1 (2412 MHz 11b)
	regdomain FCC country US authmode OPEN privacy OFF txpower 30
	bmiss 10 scanvalid 60 wme bintval 0
	parent interface: iwm0
	media: IEEE 802.11 Wireless Ethernet autoselect (autoselect)
	status: no carrier
	nd6 options=29<PERFORMNUD,IFDISABLED,AUTO_LINKLOCAL>
`

const ifconfigAssociated24GHz = `wlan0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> metric 0 mtu 1500
	ether aa:bb:cc:dd:ee:ff
	inet 10.0.0.50 netmask 0xffffff00 broadcast 10.0.0.255
	groups: wlan
	ssid "HomeWiFi" channel 6 (2437 MHz 11g)
	regdomain FCC country US authmode WPA2/802.11i privacy ON
	txpower 30 bmiss 10 scanvalid 60 wme bintval 0
	parent interface: iwm0
	media: IEEE 802.11 Wireless Ethernet MCS mode 11ng
	status: associated
`

const listStaOutput = `ADDR               AID CHAN RATE  RSSI IDLE  TXSEQ  RXSEQ CAPS  FLAG
aa:bb:cc:dd:ee:ff    1  149 130M   -42    0      0      0  EPS   AQHTER
`

const listStaWeakSignal = `ADDR               AID CHAN RATE  RSSI IDLE  TXSEQ  RXSEQ CAPS  FLAG
11:22:33:44:55:66    1    6  54M   -78    0      0      0  EP    AQH
`

const listStaEmpty = `ADDR               AID CHAN RATE  RSSI IDLE  TXSEQ  RXSEQ CAPS  FLAG
`

func TestParseSSID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSSID string
	}{
		{"associated 5GHz", ifconfigAssociated, "MyNetwork"},
		{"associated 2.4GHz", ifconfigAssociated24GHz, "HomeWiFi"},
		{"not associated", ifconfigNotAssociated, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := reSSID.FindStringSubmatch(tc.input)
			got := ""
			if len(m) > 1 {
				got = m[1]
			}
			if got != tc.wantSSID {
				t.Errorf("SSID = %q, want %q", got, tc.wantSSID)
			}
		})
	}
}

func TestParseFrequency(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMHz string
	}{
		{"5GHz", ifconfigAssociated, "5745"},
		{"2.4GHz", ifconfigAssociated24GHz, "2437"},
		{"not associated", ifconfigNotAssociated, "2412"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := reChan.FindStringSubmatch(tc.input)
			got := ""
			if len(m) > 1 {
				got = m[1]
			}
			if got != tc.wantMHz {
				t.Errorf("freq = %q, want %q", got, tc.wantMHz)
			}
		})
	}
}

func TestParseAssociatedStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"associated", ifconfigAssociated, true},
		{"not associated", ifconfigNotAssociated, false},
		{"2.4GHz associated", ifconfigAssociated24GHz, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bytes.Contains([]byte(tc.input), []byte("status: associated"))
			if got != tc.want {
				t.Errorf("associated = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseStaInfo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSignal int
		wantRate   float64
	}{
		{"good signal", listStaOutput, -42, 130.0},
		{"weak signal", listStaWeakSignal, -78, 54.0},
		{"empty", listStaEmpty, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signal, rate := parseStaInfoFrom(tc.input)
			if signal != tc.wantSignal {
				t.Errorf("signal = %d, want %d", signal, tc.wantSignal)
			}
			if rate != tc.wantRate {
				t.Errorf("rate = %f, want %f", rate, tc.wantRate)
			}
		})
	}
}

func TestRssiToQuality(t *testing.T) {
	tests := []struct {
		rssi    int
		wantMin int
		wantMax int
	}{
		{0, 0, 0},     // no signal
		{-30, 70, 70}, // excellent
		{-50, 50, 70}, // good
		{-70, 10, 70}, // fair
		{-90, 0, 50},  // poor
		{-110, 0, 0},  // minimum
		{-120, 0, 0},  // below range
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("rssi_%d", tc.rssi), func(t *testing.T) {
			got := rssiToQuality(tc.rssi)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("rssiToQuality(%d) = %d, want [%d, %d]", tc.rssi, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}
