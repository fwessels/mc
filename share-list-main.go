/*
 * Minio Client (C) 2014, 2015 Minio, Inc.
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
	"fmt"
	"time"

	"github.com/minio/cli"
	"github.com/minio/minio-xl/pkg/probe"
)

var (
	shareListFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "help, h",
			Usage: "Help of share list.",
		},
	}
)

// Share documents via URL.
var shareList = cli.Command{
	Name:   "list",
	Usage:  "List previously shared objects and folders.",
	Action: mainShareList,
	Flags:  append(shareListFlags, globalFlags...),
	CustomHelpTemplate: `NAME:
   mc share {{.Name}} COMMAND - {{.Usage}}

COMMAND:
   upload:   list previously shared access to uploads.
   download: list previously shared access to downloads.

USAGE:
   mc share {{.Name}}

EXAMPLES:
   1. List previously shared downloads, that haven't expired yet.
       $ mc share {{.Name}} download
   2. List previously shared uploads, that haven't expired yet.
       $ mc share {{.Name}} upload
`,
}

// validate command-line args.
func checkShareListSyntax(ctx *cli.Context) {
	args := ctx.Args()
	if !args.Present() || (args.First() != "upload" && args.First() != "download") {
		cli.ShowCommandHelpAndExit(ctx, "list", 1) // last argument is exit code.
	}
}

// doShareList list shared url's.
func doShareList(cmd string) *probe.Error {
	if cmd != "upload" && cmd != "download" {
		return probe.NewError(fmt.Errorf("Unknown argument ‘%s’ passed.", cmd))
	}

	// Fetch defaults.
	uploadsFile := getShareUploadsFile()
	downloadsFile := getShareDownloadsFile()

	// Load previously saved upload-shares.
	shareDB := newShareDBV1()

	// if upload - read uploads file.
	if cmd == "upload" {
		if err := shareDB.Load(uploadsFile); err != nil {
			return err.Trace(uploadsFile)
		}
	}

	// if download - read downloads file.
	if cmd == "download" {
		if err := shareDB.Load(downloadsFile); err != nil {
			return err.Trace(downloadsFile)
		}
	}

	// Print previously shared entries.
	for shareURL, share := range shareDB.Shares {
		printMsg(shareMesssage{
			ObjectURL:   share.URL,
			ShareURL:    shareURL,
			TimeLeft:    share.Expiry - time.Since(share.Date),
			ContentType: share.ContentType,
		})
	}
	return nil
}

// main entry point for share list.
func mainShareList(ctx *cli.Context) {
	// Set global flags from context.
	setGlobalsFromContext(ctx)

	// validate command-line args.
	checkShareListSyntax(ctx)

	// Additional command speific theme customization.
	shareSetColor()

	// Initialize share config folder.
	initShareConfig()

	// List shares.
	fatalIf(doShareList(ctx.Args().First()).Trace(), "Unable to list previously shared URLs.")
}
