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
	// these are instances for std synchronized writers.
	// they only need to be initialized once cause we
	// don't want writers to be racing on writing
	// to stdout and stderr.
	stdoutSyncWriter, stderrSyncWriter io.Writer
	// and this is to make sure of that.
	initializeWritersOnce sync.Once
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

// checks if the logger is configured not to log anything.
func isLevelNone(l string) bool {
	return "none" == strings.ToLower(strings.TrimSpace(l))
}

// checks if the specified level string matches to
// a valid logger level and returns it if it does,
// else it returns "AllowAll" which lets all
// logs to go through.
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

// takes a format-type string and returns a factory
// that creates a non-filtered logger with a writer.
func createLoggerFactory(loggerType string) func(io.Writer) log.Logger {
	switch strings.ToLower(strings.TrimSpace(loggerType)) {
	default:
		return log.NewJSONLogger
	}
}

// returns new synchronized stdOut & stdErr loggers based on the specified logger factory.
func createSyncStdLoggers(loggerTypeFactory func(io.Writer) log.Logger) (log.Logger, log.Logger) {

	// initialize the writers only once.
	initializeWritersOnce.Do(func() {
		if stdoutSyncWriter == nil {
			stdoutSyncWriter = log.NewSyncWriter(os.Stdout)
		}

		if stderrSyncWriter == nil {
			stderrSyncWriter = log.NewSyncWriter(os.Stderr)
		}
	})

	// now, we can use the writers to return as many loggers as we want by just calling the function.
	return log.With(loggerTypeFactory(stdoutSyncWriter), "ts", log.DefaultTimestampUTC),
		log.With(loggerTypeFactory(stderrSyncWriter), "ts", log.DefaultTimestampUTC, "caller", log.Caller(5))
}

// this is to keep track of how many log entries has been sent
// to each logger since we intend to use a separate logger
// for errors and another for the rest of the logs.
// let's call these two loggers "appenders".
type multiAppenderInstrumentedLogger struct {
	loggers map[level.Value]log.Logger
	counter metrics.Counter
	name    string
}

func (l *multiAppenderInstrumentedLogger) Log(keyvals ...interface{}) error {

	// here we loop through keys and values.
	for i := 0; i < len(keyvals); i += 2 {
		// check if this is the key that indicates the severity level of the log entry.
		if k := keyvals[i]; k == level.Key() {
			// if yes then get its value.
			if v, ok := keyvals[i+1].(level.Value); ok {
				// if we use a metrics counter then increment it for the resolved value.
				if l.counter != nil {
					l.counter.With("level", v.String()).Add(1)
				}

				// now if the loggers are defined - which they should be - get the logger
				// that matches the severity level of the log entry and append the entry
				// to that logger adding the logger name.
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

	// if you're required to log nothing, then just return a dummy logger.
	if isLevelNone(config.Level) {
		return log.NewNopLogger()
	}

	// else get the severity level required.
	lvl := getValidLevel(config.Level)

	// create two "appenders" for stdout and stderr based on the factory chosen.
	outLogger, errLogger := createSyncStdLoggers(createLoggerFactory(config.Format))

	// create a filter for the stdout "appender" based on the resolved severity level.
	outLogger = level.NewFilter(outLogger, lvl)

	// now, create a map for the defined appenders matching each severity level.
	loggers := make(map[level.Value]log.Logger)

	// errors should only go to stderr.
	loggers[level.ErrorValue()] = errLogger

	// the rest to stdout
	loggers[level.WarnValue()] = outLogger
	loggers[level.InfoValue()] = outLogger
	loggers[level.DebugValue()] = outLogger

	// finally return an instrumented wrapping logger for the appenders we've created.
	return &multiAppenderInstrumentedLogger{name: loggerName, loggers: loggers, counter: counter}
}
