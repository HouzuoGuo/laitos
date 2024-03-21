package tracing

import "fmt"

var (
	// ProcProbeCode maps all supported probe types to templated bpftrace snippets for tracing an individual process.
	ProcProbeCode = map[Probe]func(int, int) string{
		// Interval probe cannot use variable in its definition.
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
