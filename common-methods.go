/*
 * Minio Client (C) 2015 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/minio/mc/pkg/client"
	"github.com/minio/mc/pkg/client/fs"
	"github.com/minio/mc/pkg/client/s3"
	"github.com/minio/minio-xl/pkg/probe"
)

// Check if the target URL represents folder. It may or may not exist yet.
func isTargetURLDir(targetURL string) bool {
	targetURLParse := client.NewURL(targetURL)
	_, targetContent, err := url2Stat(targetURL)
	if err != nil {
		if targetURLParse.Path == string(targetURLParse.Separator) && targetURLParse.Scheme != "" {
			return false
		}
		if strings.HasSuffix(targetURLParse.Path, string(targetURLParse.Separator)) {
			return true
		}
		return false
	}
	if !targetContent.Type.IsDir() { // Target is a dir.
		return false
	}
	return true
}

// getSource gets a reader from URL.
func getSource(urlStr string) (reader io.ReadSeeker, err *probe.Error) {
	alias, urlStrFull, _, err := expandAlias(urlStr)
	if err != nil {
		return nil, err.Trace(urlStr)
	}
	return getSourceFromAlias(alias, urlStrFull)
}

// getSourceFromAlias gets a reader from URL.
func getSourceFromAlias(alias string, urlStr string) (reader io.ReadSeeker, err *probe.Error) {
	sourceClnt, err := newClientFromAlias(alias, urlStr)
	if err != nil {
		return nil, err.Trace(alias, urlStr)
	}
	return sourceClnt.Get(0, 0)
}

// putTarget writes to URL from reader. If length=-1, read until EOF.
func putTarget(urlStr string, reader io.ReadSeeker, size int64) *probe.Error {
	alias, urlStrFull, _, err := expandAlias(urlStr)
	if err != nil {
		return err
	}
	return putTargetFromAlias(alias, urlStrFull, reader, size)
}

// putTargetFromAlias writes to URL from reader. If length=-1, read until EOF.
func putTargetFromAlias(alias string, urlStr string, reader io.ReadSeeker, size int64) *probe.Error {
	targetClnt, err := newClientFromAlias(alias, urlStr)
	if err != nil {
		return err.Trace(alias, urlStr)
	}
	contentType := guessURLContentType(urlStr)
	err = targetClnt.Put(reader, size, contentType)
	if err != nil {
		return err.Trace(alias, urlStr)
	}
	return nil
}

// newClientFromAlias gives a new client interface for matching
// alias entry in the mc config file. If no matching host config entry
// is found, fs client is returned.
func newClientFromAlias(alias string, urlStr string) (client.Client, *probe.Error) {
	hostCfg := mustGetHostConfig(alias)
	if hostCfg == nil {
		// No matching host config. So we treat it like a
		// filesystem.
		fsClient, err := fs.New(urlStr)
		if err != nil {
			return nil, err.Trace(alias, urlStr)
		}
		return fsClient, nil
	}

	// We have a valid alias and hostConfig. We populate the
	// credentials from the match found in the config file.
	s3Config := new(client.Config)
	s3Config.AccessKey = hostCfg.AccessKey
	s3Config.SecretKey = hostCfg.SecretKey
	s3Config.Signature = hostCfg.API
	s3Config.AppName = "mc"
	s3Config.AppVersion = mcVersion
	s3Config.AppComments = []string{os.Args[0], runtime.GOOS, runtime.GOARCH}
	s3Config.HostURL = urlStr
	s3Config.Debug = globalDebug

	s3Client, err := s3.New(s3Config)
	if err != nil {
		return nil, err.Trace(alias, urlStr)
	}
	return s3Client, nil
}

// newClient gives a new client interface
func newClient(urlStr string) (client.Client, *probe.Error) {
	alias, urlStrFull, _, err := expandAlias(urlStr)
	if err != nil {
		return nil, err.Trace(urlStr)
	}
	return newClientFromAlias(alias, urlStrFull)
}
