package YoloCache

import (
	"fmt"
	"log"
	"testing"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGet(t *testing.T) {
	loadCounts := make(map[string]int, len(db))
	gee := NewGroup("scores", 2<<10, GetterFunc(
		// 缓存为空回回来调用回调函数，这里的判断都是在Cache中没有找到的情况下。 回调函数用来在数据源中查找缓存值（类比一下redis和mysql）
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			// 如果在数据源中找到了目标记录key
			if v, ok := db[key]; ok {
				//  loadCounts 统计某个键调用回调函数的次数，如果次数大于1，则表示调用了多次回调函数，没有缓存。
				// ok用来判断key是否存在, 如果当前key还没调用过回调函数，那么loadCounts[key]不存在，所以要先判断一下
				if _, ok := loadCounts[key]; !ok {
					// 如果不存在，将调用次数置为0
					loadCounts[key] = 0
				}
				// 调用回调函数次数+1，表示执行了一次回调函数
				loadCounts[key] += 1
				// 返回目标值给getLocally函数这里， 因为是在getLocally这个函数里调用的，所以这里的返回值会被getLocally函数接收
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	for k, v := range db {
		if view, err := gee.Get(k); err != nil || view.String() != v {
			t.Fatal("failed to get value of Tom")
		} // load from callback function
		// loadCounts[k] > 1，意味着调用了多次回调函数，没有缓存
		if _, err := gee.Get(k); err != nil || loadCounts[k] > 1 {
			t.Fatalf("cache %s miss", k)
		} // cache hit
	}

	if view, err := gee.Get("unknown"); err == nil {
		t.Fatalf("the value of unknow should be empty, but %s got", view)
	}
}
