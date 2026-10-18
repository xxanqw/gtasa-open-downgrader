// Package licenses supplies the same complete, offline-readable notice bundle
// to the downgrader and its standalone Steam launcher. See generate-notices.py.
package licenses

import _ "embed"

// Text contains the application and applicable dependency notices, plus explicit
// source-availability information and unresolved redistribution qualifications.
//
//go:embed NOTICES.txt
var Text string

const MPLSourceName = "golang-lru-v2-v2.0.7-source.zip"

// MPLSource contains unmodified v2.0.7 source, including its original licenses.
//
//go:embed golang-lru-v2-v2.0.7-source.zip
var MPLSource []byte
