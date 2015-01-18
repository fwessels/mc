/*
 * Mini Object Storage, (C) 2014 Minio, Inc.
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
	"github.com/codegangsta/cli"
)

var GetObject = cli.Command{
	Name:        "get-object",
	Usage:       "",
	Description: "Retrieves objects from Amazon S3.",
	Action:      doGetObject,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bucket",
			Value: "",
			Usage: "bucket name",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "",
			Usage: "path to Object",
		},
	},
}

var PutObject = cli.Command{
	Name:        "put-object",
	Usage:       "",
	Description: "Adds an object to a bucket.",
	Action:      doPutObject,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bucket",
			Value: "",
			Usage: "bucket name",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "",
			Usage: "Object name",
		},
		cli.StringFlag{
			Name:  "body",
			Value: "",
			Usage: "Object blob",
		},
	},
}

var ListObjects = cli.Command{
	Name:  "list-objects",
	Usage: "",
	Description: `Returns some or all (up to 1000) of the objects in a bucket.
   You can use the request parameters as selection criteria to
   return a subset of the objects in a bucket.`,
	Action: doListObjects,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bucket",
			Value: "",
			Usage: "Bucket name",
		},
	},
}

var ListBuckets = cli.Command{
	Name:  "list-buckets",
	Usage: "",
	Description: `Returns a list of all buckets owned by the authenticated
   sender of the request.`,
	Action: doListBuckets,
}

var Configure = cli.Command{
	Name:  "configure",
	Usage: "",
	Description: `Configure minio client configuration data. If your config
   file does not exist (the default location is ~/.s3auth), it will be
   automatically created for you. Note that the configure command only writes
   values to the config file. It does not use any configuration values from
   the environment variables.`,
	Action: doConfigure,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "accesskey",
			Value: "",
			Usage: "AWS access key id",
		},
		cli.StringFlag{
			Name:  "secretkey",
			Value: "",
			Usage: "AWS secret key id",
		},
	},
}

const (
	S3_AUTH = ".s3auth"
)