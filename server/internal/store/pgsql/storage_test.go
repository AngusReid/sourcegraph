// +build pgsqltest

package pgsql

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/store/testsuite"
)

var (
	storageKeyName = randomKey()
	storageValue   = fullByteRange()
)

func fullByteRange() (v []byte) {
	for i := byte(0); i < 255; i++ {
		v = append(v, i)
	}
	return
}

func randomKey() string {
	return "my-awesome\x00\x00key" + fmt.Sprint(time.Now().UnixNano())
}

func randomBucket() *sourcegraph.StorageBucket {
	return &sourcegraph.StorageBucket{
		AppName: "go-test",
		Name:    "go-test-bucket" + fmt.Sprint(time.Now().UnixNano()),
		Repo:    "github.com/foo/bar",
	}
}

// TestStorage_Get tests that Storage.Get works.
func TestStorage_Get(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()
	storageKey := &sourcegraph.StorageKey{
		Bucket: storageBucket, // TODO(slimsag): Bucket should not be nullable
		Key:    storageKeyName,
	}

	// Test that a NotFound error is returned.
	value, err := s.Get(ctx, storageKey)
	if grpc.Code(err) != codes.NotFound {
		t.Fatalf("Expected codes.NotFound, got: %+v\n", err)
	}

	// Put the first object in.
	_, err = s.Put(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: storageValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Now test that NotFound is returned for a valid bucket but an invalid key.
	value, err = s.Get(ctx, &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    storageKeyName + "-secondary",
	})
	if grpc.Code(err) != codes.NotFound {
		t.Fatalf("(2) Expected codes.NotFound, got: %+v\n", err)
	}

	// Get the object.
	value, err = s.Get(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value.Value, storageValue) {
		t.Fatalf("got %q expected %q\n", value, storageValue)
	}
}

// TestStorage_Put tests that Storage.Put works.
func TestStorage_Put(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()
	storageKey := &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    storageKeyName,
	}

	// Put the first object in.
	_, err := s.Put(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: storageValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Overwrite the value.
	newValue := []byte("new value")
	_, err = s.Put(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: newValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get the object.
	value, err := s.Get(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value.Value, newValue) {
		t.Fatalf("got %q expected %q\n", value, newValue)
	}
}

// TestStorage_PutNoOverwrite tests that Storage.PutNoOverwrite works.
func TestStorage_PutNoOverwrite(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()
	storageKey := &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    storageKeyName,
	}

	// Put the first object in.
	_, err := s.PutNoOverwrite(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: storageValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test that overwrite returns a AlreadyExists error.
	_, err = s.PutNoOverwrite(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: storageValue,
	})
	if grpc.Code(err) != codes.AlreadyExists {
		t.Fatalf("Expected codes.AlreadyExists, got: %+v\n", err)
	}
}

// TestStorage_PutNoOverwriteConcurrent tests that Storage.PutNoOverwrite works.
func TestStorage_PutNoOverwriteConcurrent(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()

	for attempt := 0; attempt < 4; attempt++ {
		// Spawn off a bunch of goroutines to try PutNoOverwrite; only one should
		// succeed.
		var (
			success    uint32
			wg         sync.WaitGroup
			storageKey = sourcegraph.StorageKey{
				Bucket: storageBucket,
				Key:    randomKey(),
			}
		)
		for g := 0; g < 10; g++ {
			wg.Add(1)
			go func() {
				_, err := s.PutNoOverwrite(ctx, &sourcegraph.StoragePutOp{
					Key:   storageKey,
					Value: storageValue,
				})
				if err == nil {
					atomic.AddUint32(&success, 1)
				} else if grpc.Code(err) != codes.AlreadyExists {
					t.Log("got error:", err)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		if success != 1 {
			t.Log("expected 1 success, got", success)
		} else {
			t.Log("got 1 success")
		}
	}
}

// TestStorage_Delete tests that Storage.Delete works.
func TestStorage_Delete(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()

	// Ensure delete on non-existant bucket is no-op.
	_, err := s.Delete(ctx, &sourcegraph.StorageKey{
		Bucket: storageBucket,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Ensure delete on non-existant key is no-op.
	_, err = s.Delete(ctx, &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    randomKey(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Put three objects in.
	keys := []string{randomKey(), randomKey(), randomKey()}
	for _, key := range keys {
		_, err = s.Put(ctx, &sourcegraph.StoragePutOp{
			Key: sourcegraph.StorageKey{
				Bucket: storageBucket,
				Key:    key,
			},
			Value: storageValue,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Delete the first object.
	first := &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    keys[0],
	}
	_, err = s.Delete(ctx, first)
	if err != nil {
		t.Fatal(err)
	}

	// Check that it no longer exists.
	exists, err := s.Exists(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if exists.Exists {
		t.Fatal("expected deleted key to no longer exist")
	}

	// Check that two objects remain.
	list, err := s.List(ctx, &sourcegraph.StorageKey{Bucket: storageBucket})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Keys) != 2 {
		t.Fatal("expect 2 keys, found", len(list.Keys))
	}

	// Delete the entire bucket
	_, err = s.Delete(ctx, &sourcegraph.StorageKey{
		Bucket: storageBucket,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check that no objects remain.
	list, err = s.List(ctx, &sourcegraph.StorageKey{Bucket: storageBucket})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Keys) != 0 {
		t.Fatal("expect 0 keys, found", len(list.Keys))
	}
}

// TesStorage_Exists tests that Storage.Exists works.
func TestStorage_Exists(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()
	storageKey := &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    storageKeyName,
	}

	// Check that no error is returned for non-existant object.
	exists, err := s.Exists(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if exists.Exists {
		t.Fatal("expected Exists == false, got Exists == true")
	}

	// Put the first object in.
	_, err = s.Put(ctx, &sourcegraph.StoragePutOp{
		Key:   *storageKey,
		Value: storageValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check that it exists.
	exists, err = s.Exists(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if !exists.Exists {
		t.Fatal("expected Exists == true, got Exists == false")
	}
}

// TestStorage_List tests that Storage.List works.
func TestStorage_List(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	storageBucket := randomBucket()
	storageKey := &sourcegraph.StorageKey{
		Bucket: storageBucket,
		Key:    storageKeyName,
	}

	// Check that no error is returned for non-existant bucket.
	list, err := s.List(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Keys) != 0 {
		t.Fatalf("expected zero keys, got %q\n", list.Keys)
	}

	// Put the objects in.
	want := []string{
		"a",
		"b",
		"c",
		storageKeyName,
	}
	for _, k := range want {
		_, err = s.Put(ctx, &sourcegraph.StoragePutOp{
			Key: sourcegraph.StorageKey{
				Bucket: storageBucket,
				Key:    k,
			},
			Value: storageValue,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Check list.
	list, err = s.List(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, list.Keys) {
		t.Fatalf("expected %q, got %q\n", want, list.Keys)
	}
}

// TestStorage_InvalidNames tests that invalid names are not allowed by the
// storage service.
func TestStorage_InvalidNames(t *testing.T) {
	ctx, done := testContext()
	defer done()

	s := &storage{}
	tests := []sourcegraph.StorageBucket{
		// Invalid bucket name tests.
		sourcegraph.StorageBucket{
			Name:    " startswithspace",
			AppName: "my-app",
			Repo:    "src.sourcegraph.com/foo/bar",
		},
		sourcegraph.StorageBucket{
			Name:    "endswithspace ",
			AppName: "my-app",
			Repo:    "src.sourcegraph.com/foo/bar",
		},
		sourcegraph.StorageBucket{
			Name:    "contains space",
			AppName: "my-app",
			Repo:    "src.sourcegraph.com/foo/bar",
		},

		// Invalid app name tests.
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: " startswithspace",
			Repo:    "src.sourcegraph.com/foo/bar",
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "endswithspace ",
			Repo:    "src.sourcegraph.com/foo/bar",
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "contains space",
			Repo:    "src.sourcegraph.com/foo/bar",
		},

		// Invalid repo URI tests.
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "my-app",
			Repo:    " starts.with.space/foo/bar",
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "my-app",
			Repo:    "ends.with.space/foo/bar ",
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "my-app",
			Repo:    "http://src.sourcegraph.com/foo/bar", // scheme not allowed
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "my-app",
			Repo:    "src.sourcegraph.com/foo/bar?ok=true", // query not allowed
		},
		sourcegraph.StorageBucket{
			Name:    "my-bucket",
			AppName: "my-app",
			Repo:    "src.sourcegraph.com/foo/bar#ok", // fragment not allowed
		},
	}

	for _, bucket := range tests {
		_, err := s.Put(ctx, &sourcegraph.StoragePutOp{
			Key: sourcegraph.StorageKey{
				Bucket: &bucket,
				Key:    storageKeyName,
			},
			Value: storageValue,
		})
		if err == nil {
			t.Logf("Put Key.Bucket: %#q\n", bucket)
			t.Fatal("expected error for non-compliant bucket name")
		}
	}
}

func TestStorage_ValidNames(t *testing.T) {
	ctx, done := testContext()
	defer done()

	testsuite.Storage_ValidNames(ctx, t, &storage{})
}
