package server

import "go.opentelemetry.io/otel/attribute"

// otelAttrString is a tiny shim so call sites don't have to import
// go.opentelemetry.io/otel/attribute directly. Helps keep the import block
// of server.go from sprawling.
func otelAttrString(k, v string) attribute.KeyValue { return attribute.String(k, v) }
func otelAttrBool(k string, v bool) attribute.KeyValue { return attribute.Bool(k, v) }
func otelAttrInt(k string, v int) attribute.KeyValue   { return attribute.Int(k, v) }
