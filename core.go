package json

// Core types have been refactored into separate files for better organization:
//
//   - types_path.go       : PathInfo, PathSegment, PropertyAccessResult, path errors
//   - types_config.go     : Config, ProcessorOptions, Stats, HealthStatus, BatchOperation
//   - types_encoding.go   : EncodeConfig and encoding constructors
//   - types_schema.go     : Schema, ValidationError, TypeSafeResult, TypeSafeAccessResult
//   - types_resource.go   : ResourceMonitor, ResourceStats
//   - types_interfaces.go : Marshaler, Unmarshaler, TextMarshaler, TextUnmarshaler
//                           and encoding/json compatible error types
//   - types_operation.go  : Operation, OperationContext, CacheKey, RateLimiter, DeletedMarker
//
// This file is kept for backward compatibility and as a documentation reference.
// All types remain in package json and maintain 100% API compatibility.
