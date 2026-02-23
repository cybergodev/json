package json

import (
	"reflect"

	"github.com/cybergodev/json/internal"
)

func (p *Processor) isArrayType(data any) bool {
	return internal.IsArrayType(data)
}

func (p *Processor) isObjectType(data any) bool {
	return internal.IsObjectType(data)
}

func (p *Processor) isMapType(data any) bool {
	return internal.IsMapType(data)
}

func (p *Processor) isSliceType(data any) bool {
	if data == nil {
		return false
	}

	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice
}

func (p *Processor) isPrimitiveType(data any) bool {
	switch data.(type) {
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return true
	default:
		return false
	}
}

// isNilOrEmpty checks if a value is nil or empty
func (p *Processor) isNilOrEmpty(data any) bool {
	return internal.IsNilOrEmpty(data)
}
