package jaws

import (
	"fmt"
	"html/template"
	"reflect"
)

type Tag string

func TagString(tag interface{}) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		} else if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}

type errTooManyTags struct{}

func (errTooManyTags) Error() string {
	return "too many tags"
}

var ErrTooManyTags = errTooManyTags{}

type errIllegalTagType struct{}

func (errIllegalTagType) Error() string {
	return "illegal tag type"
}

var ErrIllegalTagType = errIllegalTagType{}

func tagExpand(l int, rq *Request, tag interface{}, result []interface{}) ([]interface{}, error) {
	if l > 10 || len(result) > 100 {
		return result, ErrTooManyTags
	}
	switch data := tag.(type) {
	case string:
	case template.HTML:
	case int:
	case int8:
	case int16:
	case int32:
	case int64:
	case uint:
	case uint8:
	case uint16:
	case uint32:
	case uint64:
	case float32:
	case float64:
	case bool:
	case error:
	case []string:
	case []template.HTML:

	case nil:
		return result, nil
	case []Tag:
		for _, v := range data {
			result = append(result, v)
		}
		return result, nil
	case TagGetter:
		if newTag := data.JawsGetTag(rq); tag != newTag {
			return tagExpand(l+1, rq, newTag, result)
		}
		return append(result, tag), nil
	case []interface{}:
		var err error
		for _, v := range data {
			if result, err = tagExpand(l+1, rq, v, result); err != nil {
				break
			}
		}
		return result, err
	default:
		return append(result, data), nil
	}
	return result, ErrIllegalTagType
}

func TagExpand(rq *Request, tag interface{}) ([]interface{}, error) {
	return tagExpand(0, rq, tag, nil)
}

func MustTagExpand(rq *Request, tag interface{}) []interface{} {
	result, err := TagExpand(rq, tag)
	if err != nil {
		panic(err)
	}
	return result
}
