package promtail

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// LogChanSize default chan size
const LogChanSize = 5000

// LogLevel level for log
type LogLevel int

const (
	//DEBUG debug
	DEBUG LogLevel = iota
	//INFO default level
	INFO LogLevel = iota
	//WARN warning
	WARN LogLevel = iota
	//ERROR error only
	ERROR LogLevel = iota
	//DISABLE Maximum level, disables sending or printing
	DISABLE LogLevel = iota
)

//ClientConfig new config: MaxRetry
type ClientConfig struct {
	// E.g. http://localhost:3100/api/prom/push
	PushURL string
	// E.g. "{job=\"somejob\"}"
	// Labels             string
	BatchWait          time.Duration
	BatchEntriesNumber int
	// Logs are sent to Promtail if the entry level is >= SendLevel
	SendLevel LogLevel
	// Logs are printed to stdout if the entry level is >= PrintLevel
	PrintLevel LogLevel
	// MaxRetry enabled retry, default disabled, 0
	MaxRetry int
	// RetryMinWait, default 1s
	RetryMinWait time.Duration
	// RetryMaxWait default 30s
	RetryMaxWait time.Duration
}

//Client client interface
type Client interface {
	// Debugf(format string, args ...interface{})
	// Infof(format string, args ...interface{})
	// Warnf(format string, args ...interface{})
	// Errorf(format string, args ...interface{})
	// LogWithLabels write logs without Log level but with labels
	LogWithLabels(lables map[string]string, level LogLevel, line string)
	Shutdown()
}

// http.Client wrapper for adding new methods, particularly sendJsonReq
type httpClient struct {
	parent       http.Client
	RetryMax     int
	RetryMinWait time.Duration
	RetryMaxWait time.Duration
}

// A bit more convenient method for sending requests to the HTTP server
func (client *httpClient) sendJSONReq(method, url string, ctype string, reqBody []byte) (resp *http.Response, resBody []byte, err error) {

	retryclient := retryablehttp.NewClient()
	retryclient.RetryMax = client.RetryMax
	retryclient.RetryWaitMin = client.RetryMinWait
	retryclient.RetryWaitMax = client.RetryMaxWait
	// maxwait, _ := time.ParseDuration("30m")
	// retryclient.RetryWaitMax = maxwait

	httpclient := retryclient.StandardClient()

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", ctype)

	resp, err = httpclient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	resBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return resp, resBody, nil
}
