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

/*
Package logging provides custom gokit logger implementation(s).

Brief

This library provides custom gokit logger implementation(s), currently it provides JSON formatted stdout & stderr sync. implementation.

Usage

	$ go get -u github.com/adzr/logging

Then, import the package:

  import (
    "github.com/adzr/logging"
  )

Example

  logger := logging.CreateStdSyncLogger("mylogger", logging.DefaultLoggingConfiguration())
  logger.Log("key", "value")

*/
package logging
