package toolbox

import (
	"strconv"
	"testing"
	"time"
)

func TestMessageProcessor_StoreReport(t *testing.T) {
	proc := NewMessageProcessor()

	// Retrieve non-existent reports
	if reports := proc.GetLatestReports(1000); len(reports) != 0 {
		t.Fatalf("%+v", reports)
	} else if reports := proc.GetLatestReportsFromSubject("non-existent", 1000); len(reports) != 0 {
		t.Fatalf("%+v", reports)
	}

	// Store one report and retrieve
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip1",
		SubjectHostName: "subject-host-name1",
		SubjectPlatform: "subject-platform",
		ServerAddress:   "server-addr",
		ServerDaemon:    "server-daemon",
		ServerPort:      123,
	}, "ip", "daemon")

	if reports := proc.GetLatestReports(1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" {
		t.Fatalf("%+v", reports)
	}
	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" {
		t.Fatalf("%+v", reports)
	}

	// Store a report for another subject and retrieve
	time.Sleep(1100 * time.Millisecond)
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip2",
		SubjectHostName: "subject-host-name2",
		SubjectPlatform: "subject-platform",
		ServerAddress:   "server-addr",
		ServerDaemon:    "server-daemon",
		ServerPort:      123,
	}, "ip", "daemon")

	// Keep in mind that reports are retrieved from latest to oldest
	if reports := proc.GetLatestReports(1000); len(reports) != 2 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip2" || reports[1].OriginalRequest.SubjectIP != "subject-ip1" {
		t.Fatalf("%+v", reports)
	}
	if reports := proc.GetLatestReportsFromSubject("subject-host-name2", 1000); len(reports) != 1 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip2" {
		t.Fatalf("%+v", reports)
	}

	// Store a second report for the first subject and retrieve
	time.Sleep(1100 * time.Millisecond)
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "subject-ip1",
		SubjectHostName: "subject-host-name1",
		SubjectPlatform: "new-subject-platform",
		ServerAddress:   "new-server-addr",
		ServerDaemon:    "new-server-daemon",
		ServerPort:      123,
	}, "ip", "daemon")

	if reports := proc.GetLatestReports(1000); len(reports) != 3 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectIP != "subject-ip1" || reports[1].OriginalRequest.SubjectIP != "subject-ip2" || reports[2].OriginalRequest.SubjectIP != "subject-ip1" ||
		reports[0].OriginalRequest.SubjectPlatform != "new-subject-platform" || reports[2].OriginalRequest.SubjectPlatform != "subject-platform" {
		t.Fatalf("%+v", reports)
	}
	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 2 {
		t.Fatalf("%+v", reports)
	} else if reports[0].OriginalRequest.SubjectPlatform != "new-subject-platform" || reports[1].OriginalRequest.SubjectPlatform != "subject-platform" {
		t.Fatalf("%+v", reports)
	}
}

func TestMessageProcessor_EvictOldReports(t *testing.T) {
	proc := NewMessageProcessor()
	// Evict older reports (10 of them) from memory
	for i := 0; i < MaxReportsPerHostName+10; i++ {
		proc.StoreReport(SubjectReportRequest{
			SubjectIP:       strconv.Itoa(i),
			SubjectHostName: "subject-host-name1",
			SubjectPlatform: "new-subject-platform",
			ServerAddress:   "new-server-addr",
			ServerDaemon:    "new-server-daemon",
			ServerPort:      123,
		}, "ip", "daemon")
	}

	if reports := proc.GetLatestReports(2 * MaxReportsPerHostName); len(reports) != MaxReportsPerHostName {
		t.Fatal(len(reports))
	} else if latestReport := reports[0]; latestReport.OriginalRequest.SubjectIP != strconv.Itoa(MaxReportsPerHostName+10-1) {
		t.Fatalf("%+v", latestReport)
	}

	if reports := proc.GetLatestReports(2 * MaxReportsPerHostName); len(reports) != MaxReportsPerHostName {
		t.Fatal(len(reports))
	} else if latestReport := reports[0]; latestReport.OriginalRequest.SubjectIP != strconv.Itoa(MaxReportsPerHostName+10-1) {
		t.Fatalf("%+v", latestReport)
	}
}

func TestMessageProcessor_EvictExpiredReports(t *testing.T) {
	proc := NewMessageProcessor()
	// Store a report
	proc.StoreReport(SubjectReportRequest{
		SubjectIP:       "1",
		SubjectHostName: "subject-host-name1",
		SubjectPlatform: "new-subject-platform",
		ServerAddress:   "new-server-addr",
		ServerDaemon:    "new-server-daemon",
		ServerPort:      123,
	}, "ip", "daemon")
	// Change the timestamp of the report to make it expire
	(*proc.SubjectReports["subject-host-name1"])[0].StoredAt = time.Now().Add(-(SubjectExpirySecond + 1) * time.Second)

	// Store thousands of reports for an active subject, which triggers clean up in the meanwhile.
	for i := 0; i < MaxReportsPerHostName+10; i++ {
		proc.StoreReport(SubjectReportRequest{
			SubjectIP:       strconv.Itoa(i),
			SubjectHostName: "subject-host-name2",
			SubjectPlatform: "new-subject-platform",
			ServerAddress:   "new-server-addr",
			ServerDaemon:    "new-server-daemon",
			ServerPort:      123,
		}, "ip", "daemon")
	}

	if reports := proc.GetLatestReportsFromSubject("subject-host-name1", 1000); len(reports) != 0 {
		t.Fatal(len(reports))
	}
}
