// info_json.go -- JSON encoding for fio.Info
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

package fio

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// jsonInfo is the on-wire shape of an Info in JSON form. The field
// set mirrors pb.Info (and therefore fio.proto), so fio.proto stays
// the single schema of record.
//
// Three fields differ from a raw pb.Info dump:
//   - Atim/Mtim/Ctim emit as RFC3339Nano UTC strings (proto stores
//     them as int64 nanoseconds). RFC3339 is what generic tooling
//     and LLM agents expect.
//   - Mod emits as a 4-digit octal string of the permission bits
//     (e.g. "0755"). The proto carries the full uint32 including
//     Go-specific high bits (ModeDir, ModeSymlink, ...); those are
//     intentionally dropped here because they have no meaning to
//     non-Go consumers. Callers that need type information should
//     hold a typed wrapper or inspect the path.
//   - Xattr emits as a JSON object (string keys → base64 values).
//     Xattr values are arbitrary octets; base64 is the standard
//     JSON escape for opaque bytes - the same convention protojson
//     uses for proto3 `bytes`.
type jsonInfo struct {
	Ino    uint64            `json:"ino"`
	Dev    uint64            `json:"dev"`
	Rdev   uint64            `json:"rdev,omitempty"`
	Siz    int64             `json:"siz"`
	Mod    string            `json:"mod"`
	Uid    uint32            `json:"uid"`
	Gid    uint32            `json:"gid"`
	Nlink  uint32            `json:"nlink"`
	Atim   string            `json:"atim"`
	Mtim   string            `json:"mtim"`
	Ctim   string            `json:"ctim"`
	Path   string            `json:"path"`
	Blocks int64             `json:"blocks"`
	Xattr  map[string]string `json:"xattr,omitempty"`
}

// MarshalJSON implements encoding/json.Marshaler so an *Info can be
// passed directly to json.Marshal / json.Encoder.Encode.
//
// The full relative path (ii.Path()) is always emitted; the
// JunkPath MarshalFlag governs only the protobuf wire format and
// does not apply to JSON.
func (ii *Info) MarshalJSON() ([]byte, error) {
	j := jsonInfo{
		Ino:    ii.Ino,
		Dev:    ii.Dev,
		Rdev:   ii.Rdev,
		Siz:    ii.Siz,
		Mod:    fmt.Sprintf("%04o", ii.Mod.Perm()),
		Uid:    ii.Uid,
		Gid:    ii.Gid,
		Nlink:  ii.Nlink,
		Atim:   ii.Atim.UTC().Format(time.RFC3339Nano),
		Mtim:   ii.Mtim.UTC().Format(time.RFC3339Nano),
		Ctim:   ii.Ctim.UTC().Format(time.RFC3339Nano),
		Path:   ii.path,
		Blocks: ii.Blocks,
	}
	if n := len(ii.Xattr); n > 0 {
		j.Xattr = make(map[string]string, n)
		for k, v := range ii.Xattr {
			j.Xattr[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}
	}
	return json.Marshal(j)
}
