package events

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/rogpeppe/rog-go/parallel"
	"gopkg.in/inconshreveable/log15.v2"
)

const maxParallelCallbacks = 8

type eventServer struct {
	*sync.Mutex
	parallel  *parallel.Run
	callbacks map[EventID][]interface{}
}

// publish asynchronously executes each of the callbacks that are subscribed to a given event's ID.
func (s *eventServer) publish(id EventID, payload interface{}) {
	s.Lock()
	defer s.Unlock()

	pv := reflect.ValueOf(payload)
	idv := reflect.ValueOf(id)

	for _, callback := range s.callbacks[id] {
		cv := reflect.ValueOf(callback)
		// Enforce basic type safety by ensuring the first argument of the
		// callback matches the type of the event's payload.
		if cv.Type().In(1) != pv.Type() {
			log15.Warn("EventServer: event payload type does not match registered callback", "wanted", cv.Type(), "got", pv.Type())
			continue
		}

		args := []reflect.Value{idv, pv}
		go s.parallel.Do(func() error {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					log15.Warn("events: registered event handler panicked:", "error", err)
					fmt.Println("events: stack trace:", string(buf))
				}
			}()
			cv.Call(args)
			return nil
		})
	}
}

func (s *eventServer) subscribe(id EventID, callback interface{}) error {
	s.Lock()
	defer s.Unlock()

	t := reflect.TypeOf(callback)
	if t.Kind() != reflect.Func {
		return errors.New("event callback must be a func")
	}
	if t.NumIn() != 2 {
		return errors.New("event callback must have 2 arguments")
	}
	if t.In(0) != reflect.TypeOf(id) {
		return errors.New("event callback must take EventID as first argument")
	}

	if _, ok := s.callbacks[id]; !ok {
		s.callbacks[id] = make([]interface{}, 0)
	}

	s.callbacks[id] = append(s.callbacks[id], callback)
	return nil
}

func newEventServer() *eventServer {
	return &eventServer{
		Mutex:     &sync.Mutex{},
		parallel:  parallel.NewRun(maxParallelCallbacks),
		callbacks: make(map[EventID][]interface{}),
	}
}

// server is a global eventServer that is shared by all subscribers and
// publishers throughout the application. They can access the server's
// functionality via the API, which is defined as the exported methods of this
// package.
var server *eventServer

func init() {
	server = newEventServer()
}

// Publish globally broadcasts the payload of an event to all of its subscribers.
func Publish(id EventID, payload interface{}) {
	server.publish(id, payload)
}

// Subscribe stores a supplied callback that will be dispatched each time a
// given EventID is published. The callback must be a func taking two arguments:
// an EventID argument, and an argument of any type that corresponds to an event's
// payload.
func Subscribe(id EventID, callback interface{}) error {
	return server.subscribe(id, callback)
}
