package utils

import (
	"fmt"
	"reflect"
)

func Find(slice interface{}, predicate func(interface{}) bool) (interface{}, int) {
	s := reflect.ValueOf(slice)

	if s.Kind() != reflect.Slice {
		fmt.Println("Find() given a non-slice type")
		panic("Find() given a non-slice type")
	}

	for index := 0; index < s.Len(); index++ {
		if predicate(s.Index(index).Interface()) {
			return s.Index(index).Interface(), index
		}
	}

	return nil, -1
}