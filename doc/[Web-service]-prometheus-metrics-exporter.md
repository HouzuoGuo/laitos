## Introduction
Hosted by laitos [web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server), the endpoint serves metrics
information collected from the following sources in the prometheus-exporter format:

- All web service handlers: time to first byte, processing duration, size of response.
- Program resource usage: CPU time consumed, number of context switches, time spent on run queue and wait queue.
- All web proxy requests: time to first byte, connection duration, size of response.

## Configuration
Under the JSON key `HTTPHandlers`, add a string property called `PrometheusMetricsEndpoint`, value being the URL location of the service.

Keep the location a secret to yourself and make it difficult to guess. Here is an example:
<pre>
{
    ...

    "HTTPHandlers": {
        ...

        "PrometheusMetricsEndpoint": "/my-precious-metrics",

        ...
    },

    ...
}
</pre>

## Run
Modify the laitos program launch command by adding the parameter `-prominteg` to it. The parameter works as the master switch to turn on
all points of integration with prometheus:

    sudo ./laitos -prominteg -config <CONFIG FILE> -daemons ...,httpd,...

The exporter is hosted by web server, therefore remember to [run web server](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server#run).

## Usage

There are three categories of performance metrics exported by this web service:
- [Web server (httpd and insecurehttpd)](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-server) statistics are always included, such as
  individual handler's processing duration, response size, time-to-first-byte, etc.
- When you enable the [web proxy daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-web-proxy), the exporter will automatically include
  statistics such as data transfer per proxy destination, number of connections, connection duration, etc.
- When you enable the [maintenance daemon](https://github.com/HouzuoGuo/laitos/wiki/%5BDaemon%5D-system-maintenance), the exporter will automatically
  include laitos program's process statistics such as CPU usage and scheduler performance. These stats rely on Linux (`procfs`).

Next, follow the installation instructions of [prometheus](https://prometheus.io/docs/prometheus/latest/installation/) to install and start the
prometheus daemon. Feel free to run the daemon on a home desktop, a dedicated server, or on the same computer that runs laitos.

Edit the prometheus configuration file (often located at `/etc/prometheus/prometheus.yml`) and tell it to periodically fetch the exported data from
the web service's endpoint:

<pre>
...
scrape_configs:
    - job_name: 'laitos'
      scrape_interval: 20s
      scrape_timeout: 5s
      scheme: https # or https
      metrics_path: '/my-precious-metrics'
      static_configs:
          - targets: ['laitos-server.example.com:443', 'another-laitos-server.example.com:443]
</pre>

## Tips

Make the endpoint difficult to guess, this helps to prevent misuse of the service.

Visit prometheus web UI (or Grafana dashboard if they are integrated), and try out the following equations for plotting program resource usage:

- Percentage of involuntary context switches, 3-minutes running average:
  `(sum(rate(laitos_proc_num_involuntary_switches[3m])) by (instance) / (sum(rate(laitos_proc_num_involuntary_switches[3m])) by (instance) + sum(rate(laitos_proc_num_voluntary_switches[3m])) by (instance))) * 100`
- Seconds of CPU time spent by laitos server (including children) in user and kernel mode, 3-minutes running average:
  `sum(rate(laitos_proc_num_kernel_mode_sec_incl_children[3m]) + rate(laitos_proc_num_user_mode_sec_incl_children[3m])) by (instance)`
- Percentage of time spent as runnable according to OS scheduler (higher is better), 3-minutes running average:
  `(sum(rate(laitos_proc_num_run_sec[3m])) by (instance) / (sum(rate(laitos_proc_num_run_sec[3m])) by (instance) + sum(rate(laitos_proc_num_wait_sec[3m])) by (instance))) * 100`

And try out these for plotting web server stats:

- Time-to-first-byte across all handlers at 95% quantile, 3-minutes running average:
  `histogram_quantile(0.95, sum(rate(laitos_httpd_response_time_to_first_byte_seconds_bucket[3m])) by (le, instance))`
- Processing duration (including IO) across all handlers at 95% quantile, 3-minutes running average:
  `histogram_quantile(0.95, sum(rate(laitos_httpd_handler_duration_seconds_bucket[3m])) by (le, instance))`
- Size of HTTP response across all handlers at 95% quantile, 3-minutes running average:
  `histogram_quantile(0.95, sum(rate(laitos_httpd_response_size_bytes_bucket[3m])) by (le, instance))`

And try out these for plotting web proxy stats:

- Number of proxy requests per minute, 1-minute running average:
  `sum(rate(laitos_httpproxy_response_size_bytes_count[1m])) by (instance)`
- Bytes transferred to proxy clients per minute, 1-minute running average:
  `sum(rate(laitos_httpproxy_response_size_bytes_sum[1m])) by (instance)`
- Top 10 proxy destinations by data transfer (total MBs over 3hrs):
  `topk(10, sum by (host) (rate(laitos_httpproxy_response_size_bytes_sum[180m]))) * 180 * 60 / 1048576`
- Top 10 proxy destinations by num of connections (total over 3 hours):
  `topk(10, sum by (host) (rate(laitos_httpproxy_response_size_bytes_count[180m]))) * 180 * 60`
- Top 10 proxy destinations by connection duration (total seconds over 3 hours):
  `topk(10, sum by (host) (rate(laitos_httpproxy_handler_duration_seconds_sum[180m]))) * 180 * 60`
- Size of proxy response across all destinations at 90% quantile, 3-minutes running average:
  `histogram_quantile(0.90, sum(rate(laitos_httpproxy_response_size_bytes_bucket[3m])) by (le, instance))`
- Time-to-first-byte across all proxy destinations at 50% quantile, 3-minutes running average:
  `histogram_quantile(0.50, sum(rate(laitos_httpproxy_response_time_to_first_byte_seconds_bucket[3m])) by (le, instance))`
- Processing duration (including IO) across all proxy destinations at 50% quantile, 3-minutes running average:
  `histogram_quantile(0.50, sum(rate(laitos_httpproxy_handler_duration_seconds_bucket[3m])) by (le, instance))`

