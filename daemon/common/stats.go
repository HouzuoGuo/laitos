package common

import (
	"fmt"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
)

var (
	CommandStats        = misc.NewStats()
	DNSDStatsTCP        = misc.NewStats()
	DNSDStatsUDP        = misc.NewStats()
	SOCKDStatsTCP       = misc.NewStats()
	SOCKDStatsUDP       = misc.NewStats()
	SNMPStats           = misc.NewStats()
	MailCommandStats    = misc.NewStats()
	SMTPDStats          = misc.NewStats()
	HTTPDStats          = misc.NewStats()
	PlainSocketStatsTCP = misc.NewStats()
	PlainSocketStatsUDP = misc.NewStats()
	TelegramBotStats    = misc.NewStats()
	AutoUnlockStats     = misc.NewStats()
)

// GetLatestStats returns statistic information from all front-end daemons, each on their own line.
func GetLatestStats() string {
	numDecimals := 2
	factor := 1000000000.0
	return fmt.Sprintf(`Web and bot commands: %s
DNS server  TCP|UDP:  %s | %s
Sock server TCP|UDP:  %s | %s
SNMP server:          %s
Mail commands:        %s
Mail server:          %s
Web servers:          %s
Text server TCP|UDP:  %s | %s
Telegram commands:    %s
Auto-unlock events:   %s
Outstanding mails:    %d KB
`,
		CommandStats.Format(factor, numDecimals),
		DNSDStatsTCP.Format(factor, numDecimals), DNSDStatsUDP.Format(factor, numDecimals),
		SOCKDStatsTCP.Format(factor, numDecimals), SOCKDStatsUDP.Format(factor, numDecimals),
		SNMPStats.Format(factor, numDecimals),
		MailCommandStats.Format(factor, numDecimals),
		SMTPDStats.Format(factor, numDecimals),
		HTTPDStats.Format(factor, numDecimals),
		PlainSocketStatsTCP.Format(factor, numDecimals), PlainSocketStatsUDP.Format(factor, numDecimals),
		TelegramBotStats.Format(factor, numDecimals),
		AutoUnlockStats.Format(factor, numDecimals),
		inet.OutstandingMailBytes/1024)
}
