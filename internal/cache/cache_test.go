package cache

import (
	"sync"
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	c := New[string, int](50*time.Millisecond, 20*time.Millisecond)
	defer c.Stop()

	c.Set("one", 1)
	c.Set("two", 2)

	v, ok := c.Get("one")
	if !ok || v != 1 {
		t.Fatalf("expected 1, got %v (ok=%v)", v, ok)
	}

	time.Sleep(70 * time.Millisecond)

	_, ok = c.Get("one")
	if ok {
		t.Fatal("expected item to expire")
	}
}

func TestCacheDelete(t *testing.T) {
	c := New[string, string](time.Minute)
	defer c.Stop()

	c.Set("k", "v")
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected key to exist")
	}

	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestCacheConcurrent(t *testing.T) {
	c := New[int, int](time.Second)
	defer c.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			c.Set(val, val*2)
			v, ok := c.Get(val)
			if ok && v != val*2 {
				t.Errorf("got %d, want %d", v, val*2)
			}
		}(i)
	}
	wg.Wait()
}
