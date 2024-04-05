package tracing

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	// TcpAddrPortKeyRegex matches the TCP address (byte array) and TCP port from a bpftrace map key.
	TcpAddrPortKeyRegex = regexp.MustCompile(`\[([0-9,-]+)\],([0-9]+)`)
	// ProbeNames maps supported tracing probe types to their bpftrace probe names.
	ProbeNames = map[Probe][]string{
		ProbeSyscallRead:  {"tracepoint:syscalls:sys_enter_read", "tracepoint:syscalls:sys_exit_read"},
		ProbeSyscallWrite: {"tracepoint:syscalls:sys_enter_write", "tracepoint:syscalls:sys_exit_write"},
		ProbeTcpProbe:     {"tracepoint:tcp:tcp_probe"},
		ProbeBlockIO:      {"tracepoint:block:block_io_start", "tracepoint:block:block_io_done"},
	}

	// BpftraceCode maps all supported probe types to templated bpftrace code.
	BpftraceCode = map[Probe]func(int, int) string{
		// Interval probe cannot use a variable in its definition, hence it has to use a string format directive.
		// The code snippets have been last verified with bpftrace v0.20.1.
		ProbeSyscallRead: func(pid, intervalSec int) string {
			return fmt.Sprintf(`
tracepoint:syscalls:sys_enter_read /pid == %d/ {
	@fd[tid] = args->fd;
}
tracepoint:syscalls:sys_exit_read /pid == %d && @fd[tid]/ {
    if (args->ret > 0) {@read_fd[@fd[tid]] += args->ret;}
    delete(@fd[tid]);
}
interval:s:%d {
    print(@read_fd);
    clear(@read_fd);
}
`, pid, pid, intervalSec)
		},
		ProbeSyscallWrite: func(pid, intervalSec int) string {
			return fmt.Sprintf(`
tracepoint:syscalls:sys_enter_write /pid == %d/ {
	@fd[tid] = args->fd;
}
tracepoint:syscalls:sys_exit_write /pid == %d && @fd[tid]/ {
    if (args->ret > 0) {@write_fd[@fd[tid]] += args->ret;}
    delete(@fd[tid]);
}
interval:s:%d {
    print(@write_fd);
    clear(@write_fd);
}
`, pid, pid, intervalSec)
		},
		ProbeTcpProbe: func(pid, intervalSec int) string {
			return fmt.Sprintf(`
tracepoint:tcp:tcp_probe /pid == %d/ {
    @tcp_src[args->saddr, args->sport] += args->data_len;
    @tcp_dest[args->daddr, args->dport] += args->data_len;
}
interval:s:%d {
    print(@tcp_src); print(@tcp_dest);
    clear(@tcp_src); clear(@tcp_dest);
}
`, pid, intervalSec)
		},
		ProbeBlockIO: func(pid, intervalSec int) string {
			return fmt.Sprintf(`
tracepoint:block:block_io_start /pid == %d/ {
    @blkdev_sector_count[args->dev] += args->nr_sector;
    @blkdev_req[args->sector] = nsecs;
}

tracepoint:block:block_io_done /@blkdev_req[args->sector] != 0/ {
    @blkdev_dur[args->dev] += nsecs - @blkdev_req[args->sector];
    delete(@blkdev_req[args->sector]);
}

interval:s:%d {
    print(@blkdev_dur); print(@blkdev_sector_count);
    clear(@blkdev_dur); clear(@blkdev_sector_count);
}
`, pid, intervalSec)
		},
	}
)

// BpfMapRecord is a print out of map by bpftrace in JSON format.
type BpfMapRecord struct {
	Type string                    `json:"type"`
	Data map[string]map[string]int `json:"data"`
}

// BpfNetIOTrafficCounter is a TCP traffic counter decoded from bpftrace map output.
type BpfNetIOTrafficCounter struct {
	IP          net.IP
	Port        int
	ByteCounter int
}

func TcpTrafficFromBpfMap(bpfMap map[string]int) []BpfNetIOTrafficCounter {
	/*
		Sample data for localhost communication:
		{"type": "map", "data": {"@tcp_src": {"[10,0,0,11,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],11": 0, "[2,0,-89,74,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],42826": 73, "[2,0,-89,66,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],42818": 73}}}
		{"type": "map", "data": {"@tcp_dest": {"[10,0,-89,66,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],42818": 0, "[10,0,-89,74,0,0,0,0,0,0,0,0,0,0,0,0,0,0,-1,-1,127,0,0,1,0,0,0,0],42826": 0, "[2,0,0,11,127,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],11": 146}}}
	*/
	var ret []BpfNetIOTrafficCounter
	for addrPortKey, trafficBytes := range bpfMap {
		addrPort := TcpAddrPortKeyRegex.FindStringSubmatch(addrPortKey)
		if len(addrPort) != 3 {
			continue
		}
		sockAddrIn6Str := addrPort[1]
		port, _ := strconv.Atoi(addrPort[2])
		var sockAddrIn6 []byte
		for _, byteStr := range strings.Split(sockAddrIn6Str, ",") {
			byteVal, _ := strconv.Atoi(strings.TrimSpace(byteStr))
			sockAddrIn6 = append(sockAddrIn6, byte(byteVal))
		}
		if len(sockAddrIn6) != 28 {
			continue
		}
		var ipAddr net.IP
		switch sockAddrIn6[0] {
		case 2:
			ipAddr = net.IPv4(sockAddrIn6[4], sockAddrIn6[5], sockAddrIn6[6], sockAddrIn6[7])
		case 10:
			ipAddr = net.IP(sockAddrIn6[4 : 4+16])
		default:
			continue
		}
		ret = append(ret, BpfNetIOTrafficCounter{
			IP:          ipAddr,
			Port:        port,
			ByteCounter: trafficBytes,
		})

	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ByteCounter > ret[j].ByteCounter
	})
	return ret
}

type BpfSample struct {
	FDBytesRead            map[string]int
	FDBytesWritten         map[string]int
	TcpTrafficSources      []BpfNetIOTrafficCounter
	TcpTrafficDestinations []BpfNetIOTrafficCounter
	BlockDeviceIONanos     map[string]int
	BlockDeviceIOSectors   map[string]int
}
