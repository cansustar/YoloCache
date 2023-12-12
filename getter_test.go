package YoloCache

import (
	"reflect"
	"testing"
)

/*
借助 GetterFunc 的类型转换，将一个匿名回调函数转换成了接口 f Getter。
调用该接口的方法 f.Get(key string)，实际上就是在调用匿名回调函数。
*/

func TestGetter(t *testing.T) {
	//
	var f Getter = GetterFunc(func(key string) ([]byte, error) {
		// 将key转换为字节数组 x.(T) 才是类型断言
		return []byte(key), nil
	})
	except := []byte("key")
	if v, _ := f.Get("key"); !reflect.DeepEqual(v, except) {
		t.Errorf("callback failed")
	}
}
