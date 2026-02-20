package chanutil

import "io"

type WithResult[T any] interface {
	WithResult(c chan any) T
}

// SendReceive sends a command and awaits a typed result.
// Returns io.EOF if channel is closed.
func SendReceive[C WithResult[C], K any](commands chan C, msg C) (result K, err error) {

	defer func() {
		if recover() != nil {
			err = io.EOF
		}
	}()

	resp := make(chan any)
	commands <- msg.WithResult(resp)
	value := <-resp
	switch v := value.(type) {
	case error:
		err = v
	case K:
		result = v
	}

	return
}

// SendReceiveError sends a command and waits for an error only result
func SendReceiveError[C WithResult[C]](commands chan C, msg C) (err error) {
	defer func() {
		if recover() != nil {
			err = io.EOF
		}
	}()

	resp := make(chan any)
	commands <- msg.WithResult(resp)
	value := <-resp
	if value == nil {
		return nil
	}

	if e, ok := value.(error); ok {
		return e
	}

	return nil

}
