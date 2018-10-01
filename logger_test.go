/*
Copyright 2018 Ahmed Zaher

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

const (
	namespace  = "test"
	loggerName = "fake"
	metricName = "entries_total"
	subsystem  = "logger_" + loggerName
)

var (
	invalid = func(logger log.Logger) log.Logger {
		return log.WithPrefix(logger, level.Key(), &customLevelValue{name: "invalid"})
	}
	filters    = []string{"none", "error", "warn", "info", "debug", "all"}
	levels     = []func(log.Logger) log.Logger{invalid, level.Error, level.Warn, level.Info, level.Debug}
	errKeyVals = [][]int{{1, 1}, {2, 1}, {3, 1}, {4, 1}, {5, 1}}
	outKeyVals = [][]int{{2, 2}, {3, 2}, {3, 3}, {4, 2}, {4, 3}, {4, 4}, {5, 2}, {5, 3}, {5, 4}}
)

type customLevelValue struct {
	name string
}

func (v *customLevelValue) String() string { return v.name }
func (v *customLevelValue) levelVal()      {}

func simulate(filter string, lvl func(log.Logger) log.Logger, keyVals ...interface{}) {
	counter := stdprometheus.NewCounterVec(stdprometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      metricName,
		Help:      "Number of log entries for each severity level.",
	}, []string{"level"})

	if err := stdprometheus.Register(counter); err != nil {
		fmt.Fprintf(os.Stderr, "failed to register counter '%v', %v\n", strings.Join([]string{namespace, subsystem, metricName}, "_"), err.Error())
	}

	logger := CreateStdSyncLogger(loggerName, prometheus.NewCounter(counter),
		&Config{Level: filter, Format: "json"})

	if lvl != nil {
		logger = lvl(logger)
	}

	logger.Log(keyVals...)
}

func cleanPrometheusErrors(str string) string {
	prometheusError := fmt.Sprintf("failed to register counter '%v', duplicate metrics collector registration attempted\n", strings.Join([]string{namespace, "logger_" + loggerName, "entries_total"}, "_"))
	return strings.Replace(str, prometheusError, "", -1)
}

func collectLogs() (string, string) {

	stdOutReader, stdOutWriter, _ := os.Pipe()
	stdErrReader, stdErrWriter, _ := os.Pipe()

	out, err := os.Stdout, os.Stderr

	defer func(out, err *os.File) {
		os.Stdout, os.Stderr = out, err
	}(out, err)

	os.Stdout, os.Stderr = stdOutWriter, stdErrWriter

	for fi, f := range filters {
		for li, l := range levels {
			key := fmt.Sprintf("key_%v%v", fi, li)
			val := fmt.Sprintf("val_%v%v", fi, li)
			simulate(f, l, key, val)
		}
	}

	stdOutWriter.Close()
	stdErrWriter.Close()

	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, stdOutReader)
	io.Copy(&bufErr, stdErrReader)

	return bufOut.String(), cleanPrometheusErrors(bufErr.String())
}

func validateLogs(logs string, expected [][]int) error {

	scanner := bufio.NewScanner(strings.NewReader(logs))

	for _, kv := range expected {
		if !scanner.Scan() {
			return fmt.Errorf("expected more logs but none found")
		}

		record := make(map[string]string)

		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return err
		}

		if name := record["logger"]; name != loggerName {
			return fmt.Errorf("expected logger name '%v', but found '%v'", loggerName, name)
		}

		k, v := fmt.Sprintf("key_%v%v", kv[0], kv[1]), fmt.Sprintf("val_%v%v", kv[0], kv[1])

		if val := record[k]; val != v {
			return fmt.Errorf("expected key-value (%v, %v), but found (%v, %v)", k, v, k, val)
		}
	}

	if scanner.Scan() {
		return fmt.Errorf("more unexpected logs found")
	}

	return nil
}

func TestLogs(t *testing.T) {

	stdout, stderr := collectLogs()

	if err := validateLogs(stdout, outKeyVals); err != nil {
		t.Errorf("failed to validate stdout, %v", err.Error())
	}

	if err := validateLogs(stderr, errKeyVals); err != nil {
		t.Errorf("failed to validate stderr, %v", err.Error())
	}
}

func TestConfiguration(t *testing.T) {
	c := Configuration()

	if c.Format != "json" || c.Level != "info" {
		t.Errorf("default configuration expected: ('json', 'info'), but found (%v', '%v')",
			c.Format, c.Level)
	}
}
