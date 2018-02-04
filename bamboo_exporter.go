package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

// https://developer.atlassian.com/bamboodev/rest-apis/bamboo-rest-resources

type Exporter struct {
	URI        string
	HTTPClient *http.Client
	Auth       struct {
		UserName string
		Password string
	}

	up              prometheus.Gauge
	isRunning       prometheus.Gauge
	agentCountTotal prometheus.Gauge
	agentCountBusy  prometheus.Gauge
	buildQueue      prometheus.Gauge
}

type BambooAgent struct {
	Id        int64  `json:"id"`
	HostName  string `json:"name"`
	IsRemote  string `json:"type"`
	IsActive  bool   `json:"active"`
	IsEnabled bool   `json:"enabled"`
	IsWorking bool   `json:"busy"`
}

type BambooAgents []BambooAgent

func (e Exporter) GetAgents() (BambooAgents, error) {
	var output BambooAgents

	request, err := e.Do("/rest/api/latest/agent")
	if err != nil {
		return output, err
	}

	if err := json.Unmarshal(request, &output); err != nil {
		return output, err
	}

	return output, nil
}

type BambooQueue struct {
	QueuedBuilds struct {
		Size       int64 `json:"size"`
		StartIndex int64 `json:"start-index"`
		MaxResult  int64 `json:"max-result"`
	} `json:"queuedBuilds"`
}

func (e Exporter) GetQueue() (BambooQueue, error) {
	var output BambooQueue

	request, err := e.Do("/rest/api/latest/queue")
	if err != nil {
		return output, err
	}

	if err := json.Unmarshal(request, &output); err != nil {
		return output, err
	}

	return output, nil
}

type BambooVersion struct {
	Build   string `json:"buildNumber"`
	State   string `json:"state"`
	Version string `json:"version"`
}

func (e Exporter) GetVersion() (BambooVersion, error) {
	var output BambooVersion

	request, err := e.Do("/rest/api/latest/info")
	if err != nil {
		return output, err
	}

	if err := json.Unmarshal(request, &output); err != nil {
		return output, err
	}

	return output, err
}

func (e Exporter) Do(endpoint string) (output []byte, err error) {

	request, err := http.NewRequest("GET", e.URI+endpoint+"?os_authType=basic", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.SetBasicAuth(e.Auth.UserName, e.Auth.Password)

	response, err := e.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	statusCode := response.StatusCode
	if statusCode != 200 {
		return nil, errors.New("cannot access " + endpoint + " endpoint do you have correct permissions")
	}

	defer response.Body.Close()
	output, err = ioutil.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	return output, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.up.Describe(ch)
	e.isRunning.Describe(ch)
	e.agentCountTotal.Describe(ch)
	e.agentCountBusy.Describe(ch)
	e.buildQueue.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	e.up.Set(1)

	e.isRunning.Set(0)
	version, err := e.GetVersion()
	if err != nil {
		log.Errorf("Can't scrape bamboo: %v", err)
	}
	if strings.ToLower(version.State) == "running" {
		e.isRunning.Set(1)
	}
	ch <- e.isRunning

	e.agentCountTotal.Set(0)
	agentList, err := e.GetAgents()
	if err != nil {
		log.Errorf("Can't scrape bamboo: %v", err)
	}
	e.agentCountTotal.Set(float64(len(agentList)))
	ch <- e.agentCountTotal

	var busyAgents float64 = 0
	e.agentCountBusy.Set(busyAgents)

	for _, agent := range agentList {

		var (
			hostName          = strings.Replace(strings.ToLower(agent.HostName), " ", "_", -1)
			isActive          = "no"
			isEnabled         = "no"
			isRemote          = "no"
			isWorking float64 = 0
		)

		if agent.IsWorking {
			isWorking = 1
			busyAgents += 1
		}

		if agent.IsActive {
			isActive = "yes"
		}

		if agent.IsEnabled {
			isEnabled = "yes"
		}

		if strings.ToLower(agent.IsRemote) == "remote" {
			isRemote = "yes"
		}

		ch <- prometheus.MustNewConstMetric(
			buildAgent, prometheus.GaugeValue, isWorking,
			hostName, isRemote, isEnabled, isActive, strconv.FormatInt(agent.Id, 10),
		)

	}

	e.agentCountBusy.Set(busyAgents)
	ch <- e.agentCountBusy

	e.buildQueue.Set(0)
	buildQueue, err := e.GetQueue()
	if err != nil {
		log.Errorf("Can't scrape bamboo: %v", err)
	}

	e.buildQueue.Set(float64(buildQueue.QueuedBuilds.Size))
	ch <- e.buildQueue

	if err != nil {
		e.up.Set(0)
	}
	ch <- e.up

	return
}

const nameSpace = "bamboo"

var (
	version    = "dev"
	versionUrl = "https://github.com/wakeful/bamboo_exporter"

	showVersion   = flag.Bool("version", false, "show version and exit")
	uri           = flag.String("uri", "http://bamboo-uri", "bamboo uri")
	userName      = flag.String("user", "root", "bamboo user name")
	userPassword  = flag.String("password", "1234", "bamboo user password")
	listenAddress = flag.String("listen-address", ":8080", "Address on which to expose metrics.")
	metricsPath   = flag.String("telemetry-path", "/metrics", "Path under which to expose metrics.")

	buildAgent = prometheus.NewDesc(
		prometheus.BuildFQName(nameSpace, "agent", "busy"),
		"bamboo agent information",
		[]string{"hostName", "isRemote", "isEnabled", "isActive", "id"}, nil)
)

var supportedSchema = map[string]bool{
	"http":  true,
	"https": true,
}

func NewExporter(uri, user, password string) *Exporter {
	return &Exporter{uri,
		&http.Client{
			Timeout: 3 * time.Second,
		},
		struct {
			UserName string
			Password string
		}{
			UserName: user,
			Password: password,
		},
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: nameSpace,
			Name:      "up",
			Help:      "was the last scrape of bamboo successful?",
		}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: nameSpace,
			Name:      "running",
			Help:      "is bamboo running?",
		}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: nameSpace,
			Subsystem: "agent",
			Name:      "count_total",
			Help:      "number of build agents",
		}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: nameSpace,
			Subsystem: "agent",
			Name:      "count_busy",
			Help:      "number of busy build agents",
		}),
		prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: nameSpace,
			Subsystem: "queue",
			Name:      "count",
			Help:      "number of jobs in build queue",
		}),
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("bamboo_exporter\n url: %s\n version: %s\n", versionUrl, version)
		os.Exit(2)
	}

	if *uri == "" || *userName == "" || *userPassword == "" {
		log.Errorln("uri, user & password are mandatory")
		os.Exit(2)
	}

	parseURI, err := url.Parse(*uri)
	if err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
	if !supportedSchema[parseURI.Scheme] {
		log.Error("schema not supported")
		os.Exit(2)
	}

	log.Infof("starting bamboo_exporter for uri: %s on %s", *uri, *listenAddress)
	exp := NewExporter(*uri, *userName, *userPassword)

	prometheus.Unregister(prometheus.NewGoCollector())
	prometheus.Unregister(prometheus.NewProcessCollector(os.Getegid(), ""))
	prometheus.MustRegister(exp)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, *metricsPath, http.StatusMovedPermanently)
	})

	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}
