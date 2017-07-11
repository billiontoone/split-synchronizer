package latency

import (
	"fmt"
	"sync"
	"time"

	"github.com/splitio/go-agent/log"
	"github.com/splitio/go-agent/splitio"
	"github.com/splitio/go-agent/splitio/api"
	"github.com/splitio/go-agent/splitio/nethelper"
	"github.com/splitio/go-agent/splitio/recorder"
)

const maxLatency = 7481828

var buckets = []int64{1000, 1500, 2250, 3375, 5063, 7594, 11391, 17086, 25629, 38443, 57665, 86498, 129746, 194620, 291929, 437894, 656841, 985261, 1477892, 2216838, 3325257, 4987885, 7481828}

// NewLatency returns a Latency instance
func NewLatency() *Latency {
	latency := &Latency{latencies: make(map[string][]int64),
		lmutex:          &sync.Mutex{},
		recorderAdapter: recorder.MetricsHTTPRecorder{},
		postRate:        60}

	go latency.PostLatenciesWorker()

	return latency
}

type Latency struct {
	latencies       map[string][]int64
	lmutex          *sync.Mutex
	recorderAdapter recorder.MetricsRecorder
	postRate        int64
}

// StartMeasuringLatency return a checkpoint number in nanoseconds
func (l *Latency) StartMeasuringLatency() int64 {
	return time.Now().UnixNano()
}

// calculateLatency given the checkpoint number returns the elapsed microseconds
func (l *Latency) calculateLatency(timeStart int64) int64 {
	timeEnd := time.Now().UnixNano()
	return int64(float64(timeEnd-timeStart) * 0.001)
}

// getBucketForLatencyMicros returns the bucket number to increment latency
func (l *Latency) getBucketForLatencyMicros(latency int64) int {
	for k, v := range buckets {
		if latency <= v {
			return k
		}
	}
	return len(buckets) - 1
}

// RegisterLatency regists
func (l *Latency) RegisterLatency(name string, startCheckpoint int64) {
	latency := l.calculateLatency(startCheckpoint)
	bucket := l.getBucketForLatencyMicros(latency)
	l.lmutex.Lock()
	if l.latencies[name] == nil {
		l.latencies[name] = make([]int64, len(buckets))
	}
	l.latencies[name][bucket] += 1
	l.lmutex.Unlock()
}

func (l *Latency) PostLatenciesWorker() {
	for {
		fmt.Println("Running POST LATENCY WORKER...")
		select {
		case <-time.After(time.Second * time.Duration(l.postRate)):
			log.Debug.Println("Posting go proxy latencies")
		}

		l.lmutex.Lock()
		fmt.Println(l.latencies)
		var latenciesDataSet []api.LatenciesDTO
		for metricName, latencyValues := range l.latencies {
			latenciesDataSet = append(latenciesDataSet, api.LatenciesDTO{MetricName: metricName, Latencies: latencyValues})
		}
		//Dropping latencies
		l.latencies = make(map[string][]int64)

		l.lmutex.Unlock()
		sdkVersion := "goproxy-" + splitio.Version
		machineIP, err := nethelper.ExternalIP()
		if err != nil {
			machineIP = "unknown"
		}

		if len(latenciesDataSet) > 0 {
			fmt.Println("SENDING LATENCIES")
			errp := l.recorderAdapter.PostLatencies(latenciesDataSet, sdkVersion, machineIP)
			if errp != nil {
				log.Error.Println("Go-proxy latencies worker:", errp)
				log.Warning.Println(l.latencies)
			}
		}
	}
}
