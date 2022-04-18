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

// +build unit

package logger

import (
	"bytes"
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"


	dockerlogger "github.com/docker/docker/daemon/logger"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const (
	maxRetries        = 3
	testErrMsg        = "test error message"
	testContainerID   = "test-container-id"
	testContainerName = "test-container-name"
)

var (
	dummyLogMsg            = []byte("test log message")
	dummySource            = "stdout"
	dummyTime              = time.Date(2020, time.January, 14, 01, 59, 0, 0, time.UTC)
	dummyCleanupTime       = time.Duration(2 * time.Second)
	logDestinationFileName string
)

// dummyClient is only used for testing. It owns the customized Log function used in
// TestSendLogs test case as we do not need the functionality that the actual Log function
// is doing inside the test. Mock Log function is not enough here as there does not exist a
// better way to verify what happened in the TestSendLogs test, which has a goroutine.
type dummyClient struct{}

// Log implements customized workflow used for testing purpose.
// This is only trigger in TestSendLogs test case. It writes current log message to the end of
// tmp test file, which makes sure the function itself accepts and "logging" the message
// correctly.
func (d *dummyClient) Log(msg *dockerlogger.Message) error {
	_, err := os.Stat(logDestinationFileName)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(logDestinationFileName, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrapf(err,
			"unable to open file %s to record log message", logDestinationFileName)
	}
	defer f.Close()
	f.Write(msg.Line)
	f.Write([]byte{'\n'})

	return nil
}

func checkLogFile(t *testing.T, fileName string, expectedNumLines int) {
    file, err := os.Open(fileName)
    require.NoError(t, err)
    defer file.Close()

    scanner := bufio.NewScanner(file)
    lines := 0
    for scanner.Scan() {
        lines++
    }
	require.Equal(t, expectedNumLines, lines)

    err = scanner.Err(); 
    require.NoError(t, err)
}

// TestSendLogs tests sendLogs goroutine that gets log message from mock io pipe and sends
// to mock destination. In this test case, the source and destination are both tmp files that
// read from and write to inside the customized Log function.
func TestSendLogs(t *testing.T) {

	for _, tc := range []struct {
		testName           string
		bufferSizeInBytes  int
		maxReadBytes       int
		logMessages        []string
		expectedNumOfLines int
	}{
		{
			testName:          "general case",
			bufferSizeInBytes: 100,
			maxReadBytes:      80, // Larger than the sum of sizes of two log messages.
			logMessages: []string{
				"First line to write",
				"Second line to write",
			},
			expectedNumOfLines: 2, // 2 messages stay as 2 messages
		},
		{
			testName:          "long log message",
			bufferSizeInBytes: 8,
			maxReadBytes:      4,
			logMessages: []string{
				"First line to write", // Larger than buffer size.
			},
			expectedNumOfLines: 3, // One line 19 chars with 8 char buffer becomes 3 split messages
		},
		{
			testName:          "two long log messages",
			bufferSizeInBytes: 8,
			maxReadBytes:      4,
			logMessages: []string{
				"First line to write", // 19 chars => 3 messages
				"Second line to write", // 20 chars => 3 messages
			},
			expectedNumOfLines: 6, // 3 + 3 = 6 total
		},
	} {
		t.Run(tc.testName, func(t *testing.T) {
			l := &Logger{
				Info:              &dockerlogger.Info{},
				Stream:            &dummyClient{},
				bufferSizeInBytes: tc.bufferSizeInBytes,
				maxReadBytes:      tc.maxReadBytes,
			}
			// Create a tmp file that used to mock the io pipe where the logger reads log
			// messages from.
			tmpIOSource, err := ioutil.TempFile("", "")
			require.NoError(t, err)
			defer os.Remove(tmpIOSource.Name())
			var (
				expectedSize int64
				testPipe     bytes.Buffer
			)
			for _, logMessage := range tc.logMessages {
				expectedSize += int64(len([]rune(logMessage)))
				_, err := testPipe.WriteString(logMessage + "\n")
				require.NoError(t, err)
			}
			expectedSize += int64(tc.expectedNumOfLines) // for newlines

			// Create a tmp file that used to inside customized dummy Log function where the
			// logger sends log messages to.
			tmpDest, err := ioutil.TempFile(os.TempDir(), "")
			require.NoError(t, err)
			defer os.Remove(tmpDest.Name())
			logDestinationFileName = tmpDest.Name()
			t.Log(tmpDest.Name())
			t.Log("hi please work")

			var errGroup errgroup.Group
			errGroup.Go(func() error {
				return l.sendLogs(context.TODO(), &testPipe, dummySource, -1, -1, &dummyCleanupTime)
			})
			err = errGroup.Wait()
			require.NoError(t, err)

			// Make sure the new scanned log message has been written to the tmp file by sendLogs
			// goroutine.
			logDestinationInfo, err := os.Stat(logDestinationFileName)
			require.NoError(t, err)
			require.Equal(t, expectedSize, logDestinationInfo.Size())

			checkLogFile(t, logDestinationFileName, tc.expectedNumOfLines)
		})
	}
}

// TestNewInfo tests if NewInfo function creates logger info correctly.
func TestNewInfo(t *testing.T) {
	config := map[string]string{
		"configKey1": "configVal1",
		"configKey2": "configVal2",
		"configKey3": "configVal3",
	}
	info := NewInfo(testContainerID, testContainerName, WithConfig(config))
	require.Equal(t, config, info.Config)
}
