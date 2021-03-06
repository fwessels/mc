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
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/mc/pkg/client"
	"github.com/minio/minio-xl/pkg/probe"
	"github.com/minio/minio/pkg/contentdb"
)

func isURLVirtualHostStyle(hostURL string) bool {
	matchS3, _ := filepath.Match("*.s3*.amazonaws.com", hostURL)
	matchGoogle, _ := filepath.Match("*.storage.googleapis.com", hostURL)
	return matchS3 || matchGoogle
}

// urlJoinPath Join a path to existing URL.
func urlJoinPath(url1, url2 string) string {
	u1 := client.NewURL(url1)
	u2 := client.NewURL(url2)
	return client.JoinURLs(u1, u2).String()
}

// url2Stat returns stat info for URL.
func url2Stat(urlStr string) (client client.Client, content *client.Content, err *probe.Error) {
	client, err = newClient(urlStr)
	if err != nil {
		return nil, nil, err.Trace(urlStr)
	}
	content, err = client.Stat()
	if err != nil {
		return nil, nil, err.Trace(urlStr)
	}
	return client, content, nil
}

// url2Alias separates alias and path from the URL. Aliased URL is of the form [/]alias/path/to/blah. If
func url2Alias(aliasedURL string) (alias, path string) {
	urlStr := aliasedURL

	// Remove "/" prefix before alias, if any.
	urlStr = strings.TrimPrefix(urlStr, string(os.PathSeparator))

	// Remove everything after alias (i.e. after "/").
	urlParts := strings.SplitN(urlStr, string(os.PathSeparator), 2)
	if len(urlParts) == 2 {
		// Convert windows style path separator to Unix style.
		return urlParts[0], urlParts[1]
	}
	return urlParts[0], ""
}

// isURLPrefixExists - check if object key prefix exists.
func isURLPrefixExists(urlPrefix string, incomplete bool) bool {
	clnt, err := newClient(urlPrefix)
	if err != nil {
		return false
	}
	isRecursive := true
	isIncomplete := incomplete
	for entry := range clnt.List(isRecursive, isIncomplete) {
		if entry.Err != nil {
			return false
		}
		return true
	}
	return false
}

// guessURLContentType - guess content-type of the URL.
// on failure just return 'application/octet-stream'.
func guessURLContentType(urlStr string) string {
	url := client.NewURL(urlStr)
	extension := strings.TrimPrefix(filepath.Ext(url.Path), ".")
	contentType, e := contentdb.Lookup(extension)
	if e != nil {
		return "application/octet-stream"
	}
	return contentType
}
