/*
 * Minio Go Library for Amazon S3 Compatible Cloud Storage (C) 2015 Minio, Inc.
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

package minio

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PreSignV2 - presign the request in following style.
// https://${S3_BUCKET}.s3.amazonaws.com/${S3_OBJECT}?AWSAccessKeyId=${S3_ACCESS_KEY}&Expires=${TIMESTAMP}&Signature=${SIGNATURE}
func (r *Request) PreSignV2() (string, error) {
	if r.config.isAnonymous() {
		return "", errors.New("presigning cannot be done with anonymous credentials")
	}
	d := time.Now().UTC()
	// Add date if not present
	if date := r.Get("Date"); date == "" {
		r.Set("Date", d.Format(http.TimeFormat))
	}
	var path string
	// Encode URL path.
	if r.config.isVirtualHostedStyle {
		for k, v := range regions {
			if v == r.config.Region {
				path = "/" + strings.TrimSuffix(r.req.URL.Host, "."+k)
				path += r.req.URL.Path
				path = getURLEncodedPath(path)
				break
			}
		}
	} else {
		path = getURLEncodedPath(r.req.URL.Path)
	}

	// Find epoch expires when the request will expire.
	epochExpires := d.Unix() + r.expires

	// get string to sign.
	stringToSign := fmt.Sprintf("%s\n\n\n%d\n%s", r.req.Method, epochExpires, path)
	hm := hmac.New(sha1.New, []byte(r.config.SecretAccessKey))
	hm.Write([]byte(stringToSign))
	// calculate signature.
	signature := base64.StdEncoding.EncodeToString(hm.Sum(nil))

	query := r.req.URL.Query()
	// Handle specially for Google Cloud Storage.
	if r.config.Region == "google" {
		query.Set("GoogleAccessId", r.config.AccessKeyID)
	} else {
		query.Set("AWSAccessKeyId", r.config.AccessKeyID)
	}

	// Fill in Expires and Signature for presigned query.
	query.Set("Expires", strconv.FormatInt(epochExpires, 10))
	query.Set("Signature", signature)
	r.req.URL.RawQuery = query.Encode()

	return r.req.URL.String(), nil
}

// PostPresignSignatureV2 - presigned signature for PostPolicy request
func (r *Request) PostPresignSignatureV2(policyBase64 string) string {
	hm := hmac.New(sha1.New, []byte(r.config.SecretAccessKey))
	hm.Write([]byte(policyBase64))
	signature := base64.StdEncoding.EncodeToString(hm.Sum(nil))
	return signature
}

// Authorization = "AWS" + " " + AWSAccessKeyId + ":" + Signature;
// Signature = Base64( HMAC-SHA1( YourSecretAccessKeyID, UTF-8-Encoding-Of( StringToSign ) ) );
//
// StringToSign = HTTP-Verb + "\n" +
//  	Content-MD5 + "\n" +
//  	Content-Type + "\n" +
//  	Date + "\n" +
//  	CanonicalizedProtocolHeaders +
//  	CanonicalizedResource;
//
// CanonicalizedResource = [ "/" + Bucket ] +
//  	<HTTP-Request-URI, from the protocol name up to the query string> +
//  	[ subresource, if present. For example "?acl", "?location", "?logging", or "?torrent"];
//
// CanonicalizedProtocolHeaders = <described below>

// SignV2 sign the request before Do() (AWS Signature Version 2).
func (r *Request) SignV2() {
	// Initial time.
	d := time.Now().UTC()

	// Add date if not present.
	if date := r.Get("Date"); date == "" {
		r.Set("Date", d.Format(http.TimeFormat))
	}

	// Calculate HMAC for secretAccessKey.
	stringToSign := r.getStringToSignV2()
	hm := hmac.New(sha1.New, []byte(r.config.SecretAccessKey))
	hm.Write([]byte(stringToSign))

	// Prepare auth header.
	authHeader := new(bytes.Buffer)
	authHeader.WriteString(fmt.Sprintf("AWS %s:", r.config.AccessKeyID))
	encoder := base64.NewEncoder(base64.StdEncoding, authHeader)
	encoder.Write(hm.Sum(nil))
	encoder.Close()

	// Set Authorization header.
	r.req.Header.Set("Authorization", authHeader.String())
}

// From the Amazon docs:
//
// StringToSign = HTTP-Verb + "\n" +
// 	 Content-MD5 + "\n" +
//	 Content-Type + "\n" +
//	 Date + "\n" +
//	 CanonicalizedProtocolHeaders +
//	 CanonicalizedResource;
func (r *Request) getStringToSignV2() string {
	buf := new(bytes.Buffer)
	// write standard headers.
	r.writeDefaultHeaders(buf)
	// write canonicalized protocol headers if any.
	r.writeCanonicalizedHeaders(buf)
	// write canonicalized Query resources if any.
	r.writeCanonicalizedResource(buf)
	return buf.String()
}

// writeDefaultHeader - write all default necessary headers
func (r *Request) writeDefaultHeaders(buf *bytes.Buffer) {
	buf.WriteString(r.req.Method)
	buf.WriteByte('\n')
	buf.WriteString(r.req.Header.Get("Content-MD5"))
	buf.WriteByte('\n')
	buf.WriteString(r.req.Header.Get("Content-Type"))
	buf.WriteByte('\n')
	buf.WriteString(r.req.Header.Get("Date"))
	buf.WriteByte('\n')
}

// writeCanonicalizedHeaders - write canonicalized headers.
func (r *Request) writeCanonicalizedHeaders(buf *bytes.Buffer) {
	var protoHeaders []string
	vals := make(map[string][]string)
	for k, vv := range r.req.Header {
		// all the AMZ and GOOG headers should be lowercase
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz") {
			protoHeaders = append(protoHeaders, lk)
			vals[lk] = vv
		}
	}
	sort.Strings(protoHeaders)
	for _, k := range protoHeaders {
		buf.WriteString(k)
		buf.WriteByte(':')
		for idx, v := range vals[k] {
			if idx > 0 {
				buf.WriteByte(',')
			}
			if strings.Contains(v, "\n") {
				// TODO: "Unfold" long headers that
				// span multiple lines (as allowed by
				// RFC 2616, section 4.2) by replacing
				// the folding white-space (including
				// new-line) by a single space.
				buf.WriteString(v)
			} else {
				buf.WriteString(v)
			}
		}
		buf.WriteByte('\n')
	}
}

// Must be sorted:
var resourceList = []string{
	"acl",
	"location",
	"logging",
	"notification",
	"partNumber",
	"policy",
	"response-content-type",
	"response-content-language",
	"response-expires",
	"response-cache-control",
	"response-content-disposition",
	"response-content-encoding",
	"requestPayment",
	"torrent",
	"uploadId",
	"uploads",
	"versionId",
	"versioning",
	"versions",
	"website",
}

// From the Amazon docs:
//
// CanonicalizedResource = [ "/" + Bucket ] +
// 	  <HTTP-Request-URI, from the protocol name up to the query string> +
// 	  [ sub-resource, if present. For example "?acl", "?location", "?logging", or "?torrent"];
func (r *Request) writeCanonicalizedResource(buf *bytes.Buffer) error {
	requestURL := r.req.URL
	if r.config.isVirtualHostedStyle {
		for k, v := range regions {
			if v == r.config.Region {
				path := "/" + strings.TrimSuffix(requestURL.Host, "."+k)
				path += requestURL.Path
				buf.WriteString(getURLEncodedPath(path))
				break
			}
		}
	} else {
		buf.WriteString(getURLEncodedPath(requestURL.Path))
	}
	sort.Strings(resourceList)
	if requestURL.RawQuery != "" {
		var n int
		vals, _ := url.ParseQuery(requestURL.RawQuery)
		// loop through all the supported resourceList.
		for _, resource := range resourceList {
			if vv, ok := vals[resource]; ok && len(vv) > 0 {
				n++
				// first element
				switch n {
				case 1:
					buf.WriteByte('?')
				// the rest
				default:
					buf.WriteByte('&')
				}
				buf.WriteString(resource)
				// request parameters
				if len(vv[0]) > 0 {
					buf.WriteByte('=')
					buf.WriteString(url.QueryEscape(vv[0]))
				}
			}
		}
	}
	return nil
}
