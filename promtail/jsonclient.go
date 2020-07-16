package promtail

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

type jsonLogEntry struct {
	Ts   string `json:"ts"`
	Line string `json:"line"`
	// level LogLevel  // not used in JSON
}

type promtailStream struct {
	Labels  string          `json:"labels"`
	Entries []*jsonLogEntry `json:"entries"`
}

type promtailMsg struct {
	Streams []*promtailStream `json:"streams"`
}

type clientJson struct {
	config  *ClientConfig
	quit    chan struct{}
	streams chan *promtailStream
	// entries   chan *jsonLogEntry
	waitGroup sync.WaitGroup
	client    httpClient
}

func NewClientJson(conf ClientConfig) (Client, error) {
	client := clientJson{
		config:  &conf,
		quit:    make(chan struct{}),
		streams: make(chan *promtailStream, LOG_ENTRIES_CHAN_SIZE),
		// entries: make(chan *jsonLogEntry, LOG_ENTRIES_CHAN_SIZE),
		client: httpClient{},
	}

	client.waitGroup.Add(1)
	go client.run()

	return &client, nil
}

// func (c *clientJson) Debugf(format string, args ...interface{}) {
// 	c.log(format, DEBUG, "Debug: ", args...)
// }

// func (c *clientJson) Infof(format string, args ...interface{}) {
// 	c.log(format, INFO, "Info: ", args...)
// }

// func (c *clientJson) Warnf(format string, args ...interface{}) {
// 	c.log(format, WARN, "Warn: ", args...)
// }

// func (c *clientJson) Errorf(format string, args ...interface{}) {
// 	c.log(format, ERROR, "Error: ", args...)
// }

func handleLabels(labels map[string]string) string {
	var s []string
	for k, v := range labels {
		s = append(s, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	return "{" + strings.Join(s, ",") + "}"
}

func (c *clientJson) LogWithLabels(lables map[string]string, line string) {
	// v, _ := json.Marshal(lables)
	labstr := handleLabels(lables)
	now := time.Now().Format(time.RFC3339)
	// line := strings.Join(args, " ")

	entry := &jsonLogEntry{
		Ts:   now,
		Line: line,
	}

	var entries []*jsonLogEntry

	entries = append(entries, entry)

	s := promtailStream{
		Labels:  labstr,
		Entries: entries,
	}

	c.streams <- &s
}

// func (c *clientJson) log(format string, level LogLevel, prefix string, args ...interface{}) {
// 	if (level >= c.config.SendLevel) || (level >= c.config.PrintLevel) {
// 		c.entries <- &jsonLogEntry{
// 			Ts:    time.Now(),
// 			Line:  fmt.Sprintf(prefix+format, args...),
// 			level: level,
// 		}
// 	}
// }

func (c *clientJson) Shutdown() {
	close(c.quit)
	c.waitGroup.Wait()
}

func (c *clientJson) run() {
	var batch []*promtailStream
	batchSize := 0
	maxWait := time.NewTimer(c.config.BatchWait)

	defer func() {
		if batchSize > 0 {
			c.send(batch)
		}

		c.waitGroup.Done()
	}()

	for {
		select {
		case <-c.quit:
			return
		case entry := <-c.streams:
			// if entry.level >= c.config.PrintLevel {
			// 	log.Print(entry.Line)
			// }

			// if entry.level >= c.config.SendLevel {
			batch = append(batch, entry)
			batchSize++
			if batchSize >= c.config.BatchEntriesNumber {
				c.send(batch)
				batch = []*promtailStream{}
				batchSize = 0
				maxWait.Reset(c.config.BatchWait)
			}
			// }
		case <-maxWait.C:
			if batchSize > 0 {
				c.send(batch)
				batch = []*promtailStream{}
				batchSize = 0
			}
			maxWait.Reset(c.config.BatchWait)
		}
	}
}

func (c *clientJson) send(streams []*promtailStream) {
	// var streams []promtailStream
	// streams = entries
	// append(streams, promtailStream{
	// 	Labels:  c.config.Labels,
	// 	Entries: entries,
	// })

	msg := promtailMsg{Streams: streams}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("promtail.ClientJson: unable to marshal a JSON document: %s\n", err)
		return
	}

	log.Printf("posting message: %s", string(jsonMsg))

	resp, body, err := c.client.sendJsonReq("POST", c.config.PushURL, "application/json", jsonMsg)
	if err != nil {
		log.Printf("promtail.ClientJson: unable to send an HTTP request: %s\n", err)
		return
	}

	if resp.StatusCode != 204 {
		log.Printf("promtail.ClientJson: Unexpected HTTP status code: %d, message: %s\n", resp.StatusCode, body)
		return
	}
}
