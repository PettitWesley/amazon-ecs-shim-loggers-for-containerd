// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package debug

const (
	daemonName = "shim-loggers-for-containerd"
	INFO       = "info"
	ERROR      = "err"
	DEBUG      = "debug"
)

var (
	// When this set to true, logger will print more events for debugging
	Verbose   = false
	LoggerErr error
)

func DeferFuncForRunLogDriver() {
	if LoggerErr != nil {
		SendEventsToLog(daemonName, LoggerErr.Error(), ERROR, 1)
	}
}
