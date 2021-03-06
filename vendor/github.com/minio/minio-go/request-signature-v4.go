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
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// signature and API related constants.
const (
	authHeader        = "AWS4-HMAC-SHA256"
	iso8601DateFormat = "20060102T150405Z"
	yyyymmdd          = "20060102"
)

///
/// Excerpts from @lsegal - https://github.com/aws/aws-sdk-js/issues/659#issuecomment-120477258.
///
///  User-Agent:
///
///      This is ignored from signing because signing this causes problems with generating pre-signed URLs
///      (that are executed by other agents) or when customers pass requests through proxies, which may
///      modify the user-agent.
///
///  Content-Length:
///
///      This is ignored from signing because generating a pre-signed URL should not provide a content-length
///      constraint, specifically when vending a S3 pre-signed PUT URL. The corollary to this is that when
///      sending regular requests (non-pre-signed), the signature contains a checksum of the body, which
///      implicitly validates the payload length (since changing the number of bytes would change the checksum)
///      and therefore this header is not valuable in the signature.
///
///  Content-Type:
///
///      Signing this header causes quite a number of problems in browser environments, where browsers
///      like to modify and normalize the content-type header in different ways. There is more information
///      on this in https://github.com/aws/aws-sdk-js/issues/244. Avoiding this field simplifies logic
///      and reduces the possibility of future bugs
///
///  Authorization:
///
///      Is skipped for obvious reasons
///
var ignoredHeaders = map[string]bool{
	"Authorization":  true,
	"Content-Type":   true,
	"Content-Length": true,
	"User-Agent":     true,
}

// getSigningKey hmac seed to calculate final signature
func getSigningKey(secret, region string, t time.Time) []byte {
	date := sumHMAC([]byte("AWS4"+secret), []byte(t.Format(yyyymmdd)))
	regionbytes := sumHMAC(date, []byte(region))
	service := sumHMAC(regionbytes, []byte("s3"))
	signingKey := sumHMAC(service, []byte("aws4_request"))
	return signingKey
}

// getSignature final signature in hexadecimal form
func getSignature(signingKey []byte, stringToSign string) string {
	return hex.EncodeToString(sumHMAC(signingKey, []byte(stringToSign)))
}

// getScope generate a string of a specific date, an AWS region, and a service
func getScope(region string, t time.Time) string {
	scope := strings.Join([]string{
		t.Format(yyyymmdd),
		region,
		"s3",
		"aws4_request",
	}, "/")
	return scope
}

// getCredential generate a credential string
func getCredential(accessKeyID, region string, t time.Time) string {
	scope := getScope(region, t)
	return accessKeyID + "/" + scope
}

// getHashedPayload get the hexadecimal value of the SHA256 hash of the request payload
func (r *Request) getHashedPayload() string {
	if r.expires != 0 {
		return "UNSIGNED-PAYLOAD"
	}
	hashedPayload := r.req.Header.Get("X-Amz-Content-Sha256")
	return hashedPayload
}

// getCanonicalHeaders generate a list of request headers for signature.
func (r *Request) getCanonicalHeaders() string {
	var headers []string
	vals := make(map[string][]string)
	for k, vv := range r.req.Header {
		if _, ok := ignoredHeaders[http.CanonicalHeaderKey(k)]; ok {
			continue // ignored header
		}
		headers = append(headers, strings.ToLower(k))
		vals[strings.ToLower(k)] = vv
	}
	headers = append(headers, "host")
	sort.Strings(headers)

	var buf bytes.Buffer
	for _, k := range headers {
		buf.WriteString(k)
		buf.WriteByte(':')
		switch {
		case k == "host":
			buf.WriteString(r.req.URL.Host)
			fallthrough
		default:
			for idx, v := range vals[k] {
				if idx > 0 {
					buf.WriteByte(',')
				}
				buf.WriteString(v)
			}
			buf.WriteByte('\n')
		}
	}
	return buf.String()
}

// getSignedHeaders generate all signed request headers.
// i.e alphabetically sorted, semicolon-separated list of lowercase request header names
func (r *Request) getSignedHeaders() string {
	var headers []string
	for k := range r.req.Header {
		if _, ok := ignoredHeaders[http.CanonicalHeaderKey(k)]; ok {
			continue // ignored header
		}
		headers = append(headers, strings.ToLower(k))
	}
	headers = append(headers, "host")
	sort.Strings(headers)
	return strings.Join(headers, ";")
}

// getCanonicalRequest generate a canonical request of style.
//
// canonicalRequest =
//  <HTTPMethod>\n
//  <CanonicalURI>\n
//  <CanonicalQueryString>\n
//  <CanonicalHeaders>\n
//  <SignedHeaders>\n
//  <HashedPayload>
//
func (r *Request) getCanonicalRequest() string {
	r.req.URL.RawQuery = strings.Replace(r.req.URL.Query().Encode(), "+", "%20", -1)
	canonicalRequest := strings.Join([]string{
		r.req.Method,
		getURLEncodedPath(r.req.URL.Path),
		r.req.URL.RawQuery,
		r.getCanonicalHeaders(),
		r.getSignedHeaders(),
		r.getHashedPayload(),
	}, "\n")
	return canonicalRequest
}

// getStringToSign a string based on selected query values.
func (r *Request) getStringToSignV4(canonicalRequest string, t time.Time) string {
	stringToSign := authHeader + "\n" + t.Format(iso8601DateFormat) + "\n"
	stringToSign = stringToSign + getScope(r.config.Region, t) + "\n"
	stringToSign = stringToSign + hex.EncodeToString(sum256([]byte(canonicalRequest)))
	return stringToSign
}

// PreSignV4 presign the request, in accordance with
// http://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html.
func (r *Request) PreSignV4() (string, error) {
	if r.config.isAnonymous() {
		return "", errors.New("presigning cannot be done with anonymous credentials")
	}
	// Initial time.
	t := time.Now().UTC()

	// get credential string.
	credential := getCredential(r.config.AccessKeyID, r.config.Region, t)
	// get hmac signing key.
	signingKey := getSigningKey(r.config.SecretAccessKey, r.config.Region, t)

	// Get all signed headers.
	signedHeaders := r.getSignedHeaders()

	query := r.req.URL.Query()
	query.Set("X-Amz-Algorithm", authHeader)
	query.Set("X-Amz-Date", t.Format(iso8601DateFormat))
	query.Set("X-Amz-Expires", strconv.FormatInt(r.expires, 10))
	query.Set("X-Amz-SignedHeaders", signedHeaders)
	query.Set("X-Amz-Credential", credential)
	r.req.URL.RawQuery = query.Encode()

	// Get string to sign from canonical request.
	stringToSign := r.getStringToSignV4(r.getCanonicalRequest(), t)
	// calculate signature.
	signature := getSignature(signingKey, stringToSign)

	r.req.URL.RawQuery += "&X-Amz-Signature=" + signature

	return r.req.URL.String(), nil
}

// PostPresignSignatureV4 - presigned signature for PostPolicy requests.
func (r *Request) PostPresignSignatureV4(policyBase64 string, t time.Time) string {
	signingkey := getSigningKey(r.config.SecretAccessKey, r.config.Region, t)
	signature := getSignature(signingkey, policyBase64)
	return signature
}

// SignV4 sign the request before Do(), in accordance with
// http://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html.
func (r *Request) SignV4(presign bool) {
	// Initial time.
	t := time.Now().UTC()
	// Set x-amz-date.
	r.Set("X-Amz-Date", t.Format(iso8601DateFormat))

	// Get all signed headers.
	signedHeaders := r.getSignedHeaders()
	// Get string to sign from canonical request.
	stringToSign := r.getStringToSignV4(r.getCanonicalRequest(), t)

	// get credential string.
	credential := getCredential(r.config.AccessKeyID, r.config.Region, t)
	// get hmac signing key.
	signingKey := getSigningKey(r.config.SecretAccessKey, r.config.Region, t)
	// calculate signature.
	signature := getSignature(signingKey, stringToSign)

	// if regular request, construct the final authorization header.
	parts := []string{
		authHeader + " Credential=" + credential,
		"SignedHeaders=" + signedHeaders,
		"Signature=" + signature,
	}
	auth := strings.Join(parts, ", ")
	r.Set("Authorization", auth)
}
