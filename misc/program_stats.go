package misc

import (
	"fmt"

	"github.com/HouzuoGuo/laitos/lalog"
)

var (
	daemonStatsDisplayFormat = StatsDisplayFormat{DivisionFactor: 1000000000, NumDecimals: 2}

	AutoUnlockStats     = NewStats(daemonStatsDisplayFormat)
	CommandStats        = NewStats(daemonStatsDisplayFormat)
	DNSDStatsTCP        = NewStats(daemonStatsDisplayFormat)
	DNSDStatsUDP        = NewStats(daemonStatsDisplayFormat)
	HTTPDStats          = NewStats(daemonStatsDisplayFormat)
	HTTPProxyStats      = NewStats(daemonStatsDisplayFormat)
	TCPOverDNSStats     = NewStats(daemonStatsDisplayFormat)
	PlainSocketStatsTCP = NewStats(daemonStatsDisplayFormat)
	PlainSocketStatsUDP = NewStats(daemonStatsDisplayFormat)
	SerialDevicesStats  = NewStats(daemonStatsDisplayFormat)
	SimpleIPStatsTCP    = NewStats(daemonStatsDisplayFormat)
	SimpleIPStatsUDP    = NewStats(daemonStatsDisplayFormat)
	SMTPDStats          = NewStats(daemonStatsDisplayFormat)
	SNMPStats           = NewStats(daemonStatsDisplayFormat)
	SOCKDStatsTCP       = NewStats(daemonStatsDisplayFormat)
	SOCKDStatsUDP       = NewStats(daemonStatsDisplayFormat)
	TelegramBotStats    = NewStats(daemonStatsDisplayFormat)

	// OutstandingMailBytes is the total size of all outstanding mails waiting to be delivered.
	OutstandingMailBytes int64
)

// ProgramStats has the comprehensive collection of program-wide stats counters in a human-readable format.
type ProgramStats struct {
	AutoUnlock         StatsDisplayValue
	DNSOverTCP         StatsDisplayValue
	DNSOverUDP         StatsDisplayValue
	HTTP               StatsDisplayValue
	HTTPProxy          StatsDisplayValue
	TCPOverDNS         StatsDisplayValue
	PlainSocketTCP     StatsDisplayValue
	PlainSocketUDP     StatsDisplayValue
	SimpleIPServiceTCP StatsDisplayValue
	SimpleIPServiceUDP StatsDisplayValue
	SMTP               StatsDisplayValue
	SockdTCP           StatsDisplayValue
	SockdUDP           StatsDisplayValue
	TelegramBot        StatsDisplayValue

	OutgoingMailBytes int64
}

// GetLatestStats returns statistic information from all front-end daemons in a piece of multi-line, formatted text.
func GetLatestStats() string {
	return fmt.Sprintf(`Auto-unlock events        %s
Commands processed        %s
DNS server TCP|UDP        %s | %s
TCP-over-DNS proxy:       %s
HTTP/S server             %s
Plain text server TCP|UDP %s | %s
Serial port devices       %s
Simple IP servers         %s | %s
SMTP server:              %s
SNMP server:              %s
Sock server TCP|UDP:      %s | %s
Telegram commands:        %s
Mail to deliver:          %d KiloBytes
Dropped log messages:     %d
`,
		AutoUnlockStats.Format(),
		CommandStats.Format(),
		DNSDStatsTCP.Format(), DNSDStatsUDP.Format(),
		TCPOverDNSStats.Format(),
		HTTPDStats.Format(),
		PlainSocketStatsTCP.Format(), PlainSocketStatsUDP.Format(),
		SerialDevicesStats.Format(),
		SimpleIPStatsTCP.Format(), SimpleIPStatsUDP.Format(),
		SMTPDStats.Format(),
		SNMPStats.Format(),
		SOCKDStatsTCP.Format(), SOCKDStatsUDP.Format(),
		TelegramBotStats.Format(),
		OutstandingMailBytes/1024,
		lalog.NumDropped.Load(),
	)
}

// GetProgramStats returns the latest program-wide stats counters in a human-readable format.
func GetLatestDisplayValues() ProgramStats {
	return ProgramStats{
		AutoUnlock:         AutoUnlockStats.DisplayValue(),
		DNSOverTCP:         DNSDStatsTCP.DisplayValue(),
		DNSOverUDP:         DNSDStatsUDP.DisplayValue(),
		HTTP:               HTTPDStats.DisplayValue(),
		HTTPProxy:          HTTPProxyStats.DisplayValue(),
		TCPOverDNS:         TCPOverDNSStats.DisplayValue(),
		PlainSocketTCP:     PlainSocketStatsTCP.DisplayValue(),
		PlainSocketUDP:     PlainSocketStatsUDP.DisplayValue(),
		SimpleIPServiceTCP: SimpleIPStatsTCP.DisplayValue(),
		SimpleIPServiceUDP: SimpleIPStatsUDP.DisplayValue(),
		SMTP:               SMTPDStats.DisplayValue(),
		SockdTCP:           SOCKDStatsTCP.DisplayValue(),
		SockdUDP:           SOCKDStatsUDP.DisplayValue(),
		TelegramBot:        TelegramBotStats.DisplayValue(),
		OutgoingMailBytes:  OutstandingMailBytes,
	}
}
