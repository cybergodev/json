package json

import (
	"reflect"
)

func (p *Processor) deepCopyData(data any) any {
	switch v := data.(type) {
	case map[string]any:
		return p.deepCopyStringMap(v)
	case map[any]any:
		return p.deepCopyAnyMap(v)
	case []any:
		return p.deepCopyArray(v)
	case string, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return v
	default:
		return p.deepCopyReflection(data)
	}
}

func (p *Processor) deepCopyStringMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyAnyMap(data map[any]any) map[any]any {
	result := make(map[any]any)
	for key, value := range data {
		result[key] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyArray(data []any) []any {
	result := make([]any, len(data))
	for i, value := range data {
		result[i] = p.deepCopyData(value)
	}
	return result
}

func (p *Processor) deepCopyReflection(data any) any {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		newPtr := reflect.New(v.Elem().Type())
		newPtr.Elem().Set(reflect.ValueOf(p.deepCopyReflection(v.Elem().Interface())))
		return newPtr.Interface()
	case reflect.Struct:
		newStruct := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanInterface() {
				newStruct.Field(i).Set(reflect.ValueOf(p.deepCopyReflection(v.Field(i).Interface())))
			}
		}
		return newStruct.Interface()
	default:
		return data
	}
}
