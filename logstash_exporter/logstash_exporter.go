package main

import (
	"fmt"
	"flag"
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/garyburd/redigo/redis"
	"encoding/json"
	"time"
)

var (
	addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	redis_queue = flag.String("redis-queue", "logstash-stats", "Message queue between logstash and the exporter.")
	
	processedLogEntries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "logstash",
			Subsystem: "exporter",
			Name: "events_processed_total",
			Help: "Total number of events processed by logstash.",
		},
		[]string{"host", "type"},
	)
	lastLogEntry = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "logstash_events_last_event",
		Help: "xxx"})
/*
	rpcDurationsHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "rpc_durations_histogram_microseconds",
		Help:    "RPC latency distributions.",
		Buckets: prometheus.LinearBuckets(10-5*10, .5*10, 20),
	})
*/
)

func init() {
	prometheus.MustRegister(processedLogEntries)
	prometheus.MustRegister(lastLogEntry)
	//prometheus.MustRegister(rpcDurationsHistogram)
}

func main() {
	flag.Parse()
	
	go func() {
		fmt.Printf("--> Starting metric server on %s...\n", *addr)
		http.Handle("/metrics", prometheus.Handler())
		err := http.ListenAndServe(*addr, nil)
		if err != nil {
			fmt.Printf("Failed to create metric server: %s\n", err.Error())
		}
	}()

	for {
		client, err := redis.Dial("tcp", ":6379")
		if err != nil {
			panic(err)
		}
		defer client.Close()

		for {
			fmt.Printf("--> Start listening for new entires in the queue \"%s\"...\n",
				*redis_queue)
			val, err := redis.Strings(client.Do("BLPOP", *redis_queue, "0"))
			if(err != nil) {
				fmt.Printf("failed to retreive keys from redis: %s\n", err.Error())
			}
			if len(val) != 2 {
				fmt.Printf("failed to convert redis response.\n")
			}

			fmt.Printf("--> Processing new entry in the queue.\n")
			fmt.Printf("Entry in queue: %s\n", val[1])

			var m map[string]interface{}
			err = json.Unmarshal([]byte(val[1]), &m)
			if err != nil {
				panic(fmt.Sprintf("Error: %s\n", err.Error()))
			}
			//fmt.Println(m)

			// Extract the last timestamp from logstash entry
			timestamp := time.Now().UTC()
			json_timestamp := m["@timestamp"]
			if json_timestamp != nil {
				timestamp, err = time.Parse(time.RFC3339, json_timestamp.(string))
				if err != nil {
					fmt.Printf("Unable to parse timestamp: %s\n", err.Error())
					timestamp = time.Now().UTC()
				}
			}
			lastLogEntry.Set(float64(timestamp.Unix()))

			// Extract host and type from json
			host := m["host"]
			if host == nil {
				host = "n/a"
			}
			typ := m["type"]
			if typ == nil {
				typ = "n/a"
			}

			// Incement the Prometheus counter about total processed messages.
			processedLogEntries.WithLabelValues(host.(string), typ.(string)).Add(1)
		}
		
	}

}
