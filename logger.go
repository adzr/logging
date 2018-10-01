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
	"io"
	"os"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
)

const (
	// DefaultFormat is the default logging output format.
	DefaultFormat = "json"
	// DefaultLevel is the default logging severity level.
	DefaultLevel = "info"
)

var (
	syncWriterLock   sync.Mutex
	stdoutSyncWriter io.Writer
	stderrSyncWriter io.Writer
)

// Config carries service logging configuration.
type Config struct {
	// Format is the logging output format, it can be only 'json' for now, any other value will be ignored.
	Format string `json:"format"`
	// Level is the logging severity level allowed, it can be 'none', 'error', 'warn', 'info', 'debug'.
	// If set to 'none' no logs will appear.
	Level string `json:"level"`
}

// Configuration returns a new instance of the default configurations for logging.
func Configuration() *Config {
	return &Config{
		Format: "json",
		Level:  "info",
	}
}

func isLevelNone(l string) bool {
	return "none" == strings.ToLower(strings.TrimSpace(l))
}

func getValidLevel(l string) level.Option {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "error":
		return level.AllowError()
	case "warn":
		return level.AllowWarn()
	case "info":
		return level.AllowInfo()
	case "debug":
		return level.AllowDebug()
	default:
		return level.AllowAll()
	}
}

func createLoggerTypeFactory(loggerType string) func(io.Writer) log.Logger {
	switch strings.ToLower(strings.TrimSpace(loggerType)) {
	default:
		return log.NewJSONLogger
	}
}

func assertWriters() {
	defer syncWriterLock.Unlock()
	syncWriterLock.Lock()

	if stdoutSyncWriter == nil {
		stdoutSyncWriter = log.NewSyncWriter(os.Stdout)
	}

	if stderrSyncWriter == nil {
		stderrSyncWriter = log.NewSyncWriter(os.Stderr)
	}
}

func createSyncStdLoggers(loggerTypeFactory func(io.Writer) log.Logger) (log.Logger, log.Logger) {
	assertWriters()
	return log.With(loggerTypeFactory(stdoutSyncWriter), "ts", log.DefaultTimestampUTC),
		log.With(loggerTypeFactory(stderrSyncWriter), "ts", log.DefaultTimestampUTC, "caller", log.Caller(5))
}

type multiAppenderInstrumentedLogger struct {
	loggers map[level.Value]log.Logger
	counter metrics.Counter
	name    string
}

func (l *multiAppenderInstrumentedLogger) Log(keyvals ...interface{}) error {

	for i := 0; i < len(keyvals); i += 2 {
		if k := keyvals[i]; k == level.Key() {
			if v, ok := keyvals[i+1].(level.Value); ok {
				if l.counter != nil {
					l.counter.With("level", v.String()).Add(1)
				}

				if l.loggers != nil {
					if target := l.loggers[v.(level.Value)]; target != nil {
						keyvals = append(keyvals, "logger", l.name)
						return target.Log(keyvals...)
					}
				}
			}
			break
		}
	}

	return nil
}

// CreateStdSyncLogger returns an instance of stdout & stderr instrumented logger.
// If configuration level is set to 'none' then neither
// logs nor monitoring will take place.
func CreateStdSyncLogger(loggerName string, counter metrics.Counter, config *Config) log.Logger {

	if isLevelNone(config.Level) {
		return log.NewNopLogger()
	}

	lvl := getValidLevel(config.Level)

	// Create two loggers for stdout and stderr based on logger type chosen.
	outLogger, errLogger := createSyncStdLoggers(createLoggerTypeFactory(config.Format))

	outLogger = level.NewFilter(outLogger, lvl)

	loggers := make(map[level.Value]log.Logger)

	loggers[level.ErrorValue()] = errLogger
	loggers[level.WarnValue()] = outLogger
	loggers[level.InfoValue()] = outLogger
	loggers[level.DebugValue()] = outLogger

	return &multiAppenderInstrumentedLogger{name: loggerName, loggers: loggers, counter: counter}
}
