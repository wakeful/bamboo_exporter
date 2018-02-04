# Bamboo exporter

A [Prometheus](https://prometheus.io/) exporter that collects [Bamboo](https://www.atlassian.com/software/bamboo) metrics.

### Usage

```sh
$ ./bamboo_exporter -h
Usage of ./bamboo_exporter:
  -listen-address string
        Address on which to expose metrics. (default ":8080")
  -password string
        bamboo user password (default "1234")
  -telemetry-path string
        Path under which to expose metrics. (default "/metrics")
  -uri string
        bamboo uri (default "http://bamboo-uri")
  -user string
        bamboo user name (default "root")
  -version
        show version and exit
```

## Metrics

```
# HELP bamboo_agent_busy bamboo agent information
# TYPE bamboo_agent_busy gauge
bamboo_agent_busy{hostName="deployment1",id="4124123",isActive="yes",isEnabled="yes",isRemote="no"} 0
bamboo_agent_busy{hostName="build1.dev.48k.io",id="4124113",isActive="no",isEnabled="yes",isRemote="yes"} 0
bamboo_agent_busy{hostName="build2.dev.48k.io",id="4122123",isActive="yes",isEnabled="yes",isRemote="yes"} 0
bamboo_agent_busy{hostName="build3.dev.48k.io",id="4124125",isActive="yes",isEnabled="yes",isRemote="yes"} 1
# HELP bamboo_agent_count_busy number of busy build agents
# TYPE bamboo_agent_count_busy gauge
bamboo_agent_count_busy 1
# HELP bamboo_agent_count_total number of build agents
# TYPE bamboo_agent_count_total gauge
bamboo_agent_count_total 4
# HELP bamboo_queue_count number of jobs in build queue
# TYPE bamboo_queue_count gauge
bamboo_queue_count 2
# HELP bamboo_running is bamboo running?
# TYPE bamboo_running gauge
bamboo_running 1
# HELP bamboo_up was the last scrape of bamboo successful?
# TYPE bamboo_up gauge
bamboo_up 1
```
