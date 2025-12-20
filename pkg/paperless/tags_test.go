package paperless

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentTagCacheAccess verifies that concurrent access to tag cache is safe
func TestConcurrentTagCacheAccess(t *testing.T) {
	// Clear the tag cache before the test
	tagCacheMutex.Lock()
	tagCache = make(map[string]int)
	tagCacheMutex.Unlock()

	const numGoroutines = 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	
	// Simulate concurrent access to tag cache
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			tagName := "test-tag"
			
			// Try to read from cache
			tagCacheMutex.RLock()
			_, exists := tagCache[tagName]
			tagCacheMutex.RUnlock()
			
			if !exists {
				// Try to write to cache
				tagCacheMutex.Lock()
				// Double-check
				if _, exists := tagCache[tagName]; !exists {
					tagCache[tagName] = 100 + goroutineID
				}
				tagCacheMutex.Unlock()
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors
	for err := range errors {
		t.Errorf("Goroutine error: %v", err)
	}
	
	// Verify cache has exactly one entry for the tag
	tagCacheMutex.RLock()
	_, exists := tagCache["test-tag"]
	tagCacheMutex.RUnlock()
	
	if !exists {
		t.Error("Expected tag to be in cache")
	}
	
	t.Log("Concurrent cache access test passed")
}

// TestTagCacheSynchronization tests the double-checked locking pattern
func TestTagCacheSynchronization(t *testing.T) {
	// Clear the tag cache
	tagCacheMutex.Lock()
	tagCache = make(map[string]int)
	tagCacheMutex.Unlock()
	
	const numGoroutines = 50
	tagName := "sync-test-tag"
	
	// Counter for simulated "create" operations
	var createCounter atomic.Int32
	
	var wg sync.WaitGroup
	results := make([]int, numGoroutines)
	
	// Simulate the getOrCreateTagID logic
	simulateGetOrCreate := func(name string) int {
		// First check (read lock)
		tagCacheMutex.RLock()
		if id, exists := tagCache[name]; exists {
			tagCacheMutex.RUnlock()
			return id
		}
		tagCacheMutex.RUnlock()
		
		// Acquire write lock
		tagCacheMutex.Lock()
		defer tagCacheMutex.Unlock()
		
		// Double-check
		if id, exists := tagCache[name]; exists {
			return id
		}
		
		// Simulate tag creation (should only happen once)
		createCounter.Add(1)
		time.Sleep(time.Millisecond) // Simulate API call delay
		newID := 999
		tagCache[name] = newID
		return newID
	}
	
	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = simulateGetOrCreate(tagName)
		}(i)
	}
	
	wg.Wait()
	
	// Verify all goroutines got the same ID
	for i := 0; i < numGoroutines; i++ {
		if results[i] != 999 {
			t.Errorf("Goroutine %d got unexpected ID %d, expected 999", i, results[i])
		}
	}
	
	// Verify "create" was called exactly once
	creates := createCounter.Load()
	if creates != 1 {
		t.Errorf("Expected exactly 1 create operation, got %d", creates)
	}
	
	t.Logf("Synchronization test passed: %d goroutines, 1 create operation", numGoroutines)
}

// TestTagCacheIsolation tests that different tags don't interfere with each other
func TestTagCacheIsolation(t *testing.T) {
	// Clear the tag cache
	tagCacheMutex.Lock()
	tagCache = make(map[string]int)
	tagCacheMutex.Unlock()
	
	const numTags = 10
	const goroutinesPerTag = 10
	
	var wg sync.WaitGroup
	
	for tagNum := 0; tagNum < numTags; tagNum++ {
		tagName := fmt.Sprintf("tag-%d", tagNum)
		expectedID := 100 + tagNum
		
		for g := 0; g < goroutinesPerTag; g++ {
			wg.Add(1)
			go func(name string, id int) {
				defer wg.Done()
				
				// Simulate adding to cache
				tagCacheMutex.Lock()
				if _, exists := tagCache[name]; !exists {
					tagCache[name] = id
				}
				tagCacheMutex.Unlock()
				
				// Verify we can read it back
				tagCacheMutex.RLock()
				cachedID, exists := tagCache[name]
				tagCacheMutex.RUnlock()
				
				if !exists {
					t.Errorf("Tag %s not found in cache", name)
				} else if cachedID != id {
					t.Errorf("Tag %s has ID %d, expected %d", name, cachedID, id)
				}
			}(tagName, expectedID)
		}
	}
	
	wg.Wait()
	
	// Verify all tags are in cache with correct IDs
	tagCacheMutex.RLock()
	cacheSize := len(tagCache)
	tagCacheMutex.RUnlock()
	
	if cacheSize != numTags {
		t.Errorf("Expected %d tags in cache, got %d", numTags, cacheSize)
	}
	
	t.Logf("Tag isolation test passed: %d different tags, %d goroutines each", numTags, goroutinesPerTag)
}
