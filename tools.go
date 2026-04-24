// tools.go -- pins the proto generator tools into go.sum
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim is made to its
// suitability for any purpose.
//
// Blank-imports the protoc generator binaries so `go mod tidy`
// keeps them resolved in go.sum. scripts/gen-proto.sh builds the
// two binaries when regenerating the proto output.

//go:build tools

package tools

import (
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto"
)
