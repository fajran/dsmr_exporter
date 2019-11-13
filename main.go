package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tarm/serial"
)

var port string
var addr string
var reval = regexp.MustCompile(`.+\(([0-9.]+)[^0-9.)]+`)

func init() {
	flag.StringVar(&port, "port", "/dev/ttyUSB0", "Serial port, default: /dev/ttyUSB0")
	flag.StringVar(&addr, "addr", ":8080", "Metric server listen address, default: :8080")
}

func main() {
	flag.Parse()

	log.Printf("Running metric server on %s", addr)
	log.Printf("Reading meter data from %s", port)

	c := &serial.Config{
		Name:        port,
		Baud:        115200,
		Parity:      serial.ParityNone,
		ReadTimeout: 5 * time.Second,
	}

	dr := &dataReader{Config: c}
	prometheus.MustRegister(dr)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(addr, nil))
}

type Entry struct {
	Timestamp         time.Time
	ElectricityLow    float64
	ElectricityNormal float64
	Gas               float64
}

type dataReader struct {
	Config *serial.Config

	electricityDesc *prometheus.Desc
	gasDesc         *prometheus.Desc
}

func (dr *dataReader) Read() <-chan Entry {
	s, err := serial.OpenPort(dr.Config)
	if err != nil {
		log.Fatal(err)
	}

	stopped := false
	lines := make(chan string, 100)
	go func() {
		defer close(lines)
		defer s.Close()
		scanner := bufio.NewScanner(s)
		for scanner.Scan() && !stopped {
			lines <- strings.TrimSpace(scanner.Text())
		}
	}()

	entries := make(chan Entry)
	go func() {
		defer close(entries)

		started := false
		block := make([]string, 0)
		for line := range lines {
			if strings.HasPrefix(line, "/ISK5") {
				started = true
			} else if started && strings.HasPrefix(line, "!") {
				entry, err := dr.parseEntry(block)
				if err == nil {
					entries <- entry
					break
				}
				block = make([]string, 0)
				started = false
			} else if started {
				block = append(block, line)
			}
		}
		stopped = true
	}()

	return entries
}

func (dr *dataReader) readValue(line string) float64 {
	g := reval.FindStringSubmatch(line)
	if len(g) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(g[1], 64)
	if err != nil {
		return 0
	}
	return v
}

func (dr *dataReader) parseEntry(lines []string) (Entry, error) {
	entry := Entry{}
	if len(lines) == 0 {
		return entry, fmt.Errorf("Empty lines")
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "1-0:1.8.1") {
			entry.ElectricityLow = dr.readValue(line)
		} else if strings.HasPrefix(line, "1-0:1.8.2") {
			entry.ElectricityNormal = dr.readValue(line)
		} else if strings.HasPrefix(line, "0-1:24.2.1") {
			entry.Gas = dr.readValue(line)
		}
	}
	entry.Timestamp = time.Now()

	return entry, nil
}

func (dr *dataReader) initDesc() {
	if dr.electricityDesc == nil {
		dr.electricityDesc = prometheus.NewDesc("energy_electricity_total", "Electricity usage",
			[]string{"type"}, nil)
	}
	if dr.gasDesc == nil {
		dr.gasDesc = prometheus.NewDesc("energy_gas_total", "Gas usage", nil, nil)
	}
}

func (dr *dataReader) Describe(ch chan<- *prometheus.Desc) {
	if dr.electricityDesc == nil || dr.gasDesc == nil {
		dr.initDesc()
	}

	ch <- dr.electricityDesc
	ch <- dr.gasDesc
}

func (dr *dataReader) Collect(ch chan<- prometheus.Metric) {
	if dr.electricityDesc == nil || dr.gasDesc == nil {
		dr.initDesc()
	}

	entry := <-dr.Read()
	ch <- prometheus.MustNewConstMetric(
		dr.electricityDesc,
		prometheus.CounterValue,
		entry.ElectricityLow,
		"low",
	)
	ch <- prometheus.MustNewConstMetric(
		dr.electricityDesc,
		prometheus.CounterValue,
		entry.ElectricityNormal,
		"normal",
	)
	ch <- prometheus.MustNewConstMetric(
		dr.gasDesc,
		prometheus.CounterValue,
		entry.Gas,
	)
}
